package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"yoinkdl/internal/daemon"
	"yoinkdl/internal/queue"
	"yoinkdl/internal/transfer"
)

func TestQueueCLILostAndInvalidAcknowledgmentsKeepSubmittedID(t *testing.T) {
	for _, mode := range []string{"disconnect", "remote unknown", "malformed", "wrong ID"} {
		for _, jsonMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%v", mode, jsonMode), func(t *testing.T) {
				paths := queueEnv(t, false)
				if err := os.Mkdir(paths.Runtime, 0o700); err != nil {
					t.Fatal(err)
				}
				listener, err := net.ListenUnix("unix", &net.UnixAddr{Net: "unix", Name: paths.Socket()})
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				if err := os.Chmod(paths.Socket(), 0o600); err != nil {
					t.Fatal(err)
				}
				ids := make(chan string, 1)
				go func() {
					conn, err := listener.AcceptUnix()
					if err != nil {
						ids <- ""
						return
					}
					defer conn.Close()
					conn.SetDeadline(time.Now().Add(time.Second))
					data, err := bufio.NewReader(conn).ReadBytes('\n')
					var req struct {
						Args struct {
							ID string `json:"id"`
						} `json:"args"`
					}
					if err != nil || json.Unmarshal(data, &req) != nil {
						ids <- ""
						return
					}
					ids <- req.Args.ID
					switch mode {
					case "remote unknown":
						fmt.Fprintln(conn, `{"version":1,"ok":false,"result":null,"error":{"code":"outcome_unknown","message":"queue mutation outcome is unknown; inspect queue state"}}`)
					case "malformed":
						fmt.Fprintln(conn, `{"source":"https://example.org/?token=private-token"}`)
					case "wrong ID":
						data, _ := json.Marshal(queue.JobView{ID: strings.Repeat("0", 32), Order: 1, State: queue.Queued, Output: "/tmp/file"})
						fmt.Fprintf(conn, "{\"version\":1,\"ok\":true,\"result\":%s,\"error\":null}\n", data)
					}
				}()
				args := []string{"add", "https://example.org/file?token=private-token"}
				if jsonMode {
					args = append(args, "--json")
				}
				var out, errOut bytes.Buffer
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				code := Run(ctx, args, &out, &errOut, "test")
				id := <-ids
				if code != 1 || !jobID(id) || strings.Contains(fmt.Sprint(&out, &errOut), "private-token") {
					t.Fatalf("uncertain result: code=%d, id=%q, stdout=%q, stderr=%q", code, id, &out, &errOut)
				}
				if jsonMode {
					var record queueRecord
					if err := json.Unmarshal(out.Bytes(), &record); err != nil || record.ID == nil || *record.ID != id ||
						record.Error == nil || record.Error.Code != "outcome_unknown" || record.ExitCode != 1 || string(record.Result) != "null" {
						t.Fatalf("uncertain JSON = %+v, %v", record, err)
					}
				} else if out.Len() != 0 || !strings.Contains(errOut.String(), "Job ID: "+id) || !strings.Contains(errOut.String(), "outcome is unknown") {
					t.Fatalf("uncertain text = %q / %q", &out, &errOut)
				}
			})
		}
	}
}

func TestQueueCLIOutputFailuresPreserveAcceptedJobs(t *testing.T) {
	for _, jsonMode := range []bool{false, true} {
		t.Run(fmt.Sprint(jsonMode), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "complete") }))
			t.Cleanup(server.Close)
			queueEnv(t, false)
			owner := startQueueCLI(t)
			path := filepath.Join(t.TempDir(), "file")
			args := []string{"add", server.URL + "?token=private-token", "-o", path, "-c", "1"}
			if jsonMode {
				args = append(args, "--json")
			}
			writer := writeFunc(func([]byte) (int, error) { return 0, errors.New("private-token") })
			var errOut bytes.Buffer
			if code := Run(context.Background(), args, writer, &errOut, "test"); code != 1 ||
				!strings.Contains(errOut.String(), "operation was accepted") || strings.Contains(errOut.String(), "private-token") {
				t.Fatalf("output failure: code=%d, stderr=%q", code, &errOut)
			}
			view := waitQueueCLI(t, func(v daemon.List) bool { return v.Jobs[0].State == queue.Completed })
			if !strings.Contains(errOut.String(), "Job ID: "+view.Jobs[0].ID) {
				t.Fatal("failed acknowledgment lost its job ID")
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != "complete" {
				t.Fatal("broken acknowledgment canceled the durable job")
			}
			queueRun(t, "daemon", "stop")
			if waitQueueOwner(t, owner) != 0 {
				t.Fatal("daemon did not stop cleanly")
			}
		})
	}
}

func TestQueueCLIBlockedOutputCancellationDoesNotCancelJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10")
		fmt.Fprint(w, "abc")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)
	queueEnv(t, false)
	owner := startQueueCLI(t)
	reader, writer := net.Pipe()
	defer reader.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	var errOut bytes.Buffer
	path := filepath.Join(t.TempDir(), "file")
	go func() {
		done <- Run(ctx, []string{"add", server.URL, "-o", path, "-c", "1", "--json"}, writer, &errOut, "test")
	}()
	view := waitQueueCLI(t, func(v daemon.List) bool { return len(v.Jobs) == 1 && v.Jobs[0].Bytes != nil && *v.Jobs[0].Bytes == 3 })
	cancel()
	select {
	case code := <-done:
		if code != 130 || !strings.Contains(errOut.String(), "Job ID: "+view.Jobs[0].ID) {
			t.Fatalf("blocked output cancellation: code=%d, stderr=%q", code, &errOut)
		}
	case <-time.After(time.Second):
		t.Fatal("client did not interrupt its blocked output")
	}
	waitQueueCLI(t, func(v daemon.List) bool {
		if v.Jobs[0].State != queue.Running {
			t.Fatal("client interruption canceled daemon-owned work")
		}
		return true
	})
	queueRun(t, "daemon", "stop")
	if waitQueueOwner(t, owner) != 0 {
		t.Fatal("daemon did not stop cleanly")
	}
}

func TestDaemonCLIStartupOutputFailureDoesNotDispatch(t *testing.T) {
	paths := queueEnv(t, false)
	if err := os.MkdirAll(filepath.Dir(paths.State), 0o700); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	t.Cleanup(server.Close)
	store, err := queue.Open(paths.State)
	if err != nil {
		t.Fatal(err)
	}
	j, err := queue.NewJob(strings.Repeat("a", 32), 1, transfer.Request{URL: server.URL, Output: filepath.Join(paths.State, "file")})
	if err != nil {
		t.Fatal(err)
	}
	state := queue.NewSnapshot()
	state.Jobs, state.NextOrder = []queue.Job{j}, 2
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	store.Close()
	writer := writeFunc(func([]byte) (int, error) { return 0, io.ErrClosedPipe })
	var errOut bytes.Buffer
	if code := Run(context.Background(), []string{"daemon", "start", "--parallel", "2"}, writer, &errOut, "test"); code != 1 {
		t.Fatalf("startup output failure returned %d", code)
	}
	store, err = queue.Open(paths.State)
	if err != nil {
		t.Fatal("failed startup retained ownership", err)
	}
	defer store.Close()
	state, err = store.Snapshot()
	if err != nil || state.Parallel != 2 || state.Jobs[0].State != queue.Queued || calls.Load() != 0 {
		t.Fatal("failed startup dispatched work or rolled back accepted settings")
	}
	if _, err := os.Lstat(paths.Socket()); !os.IsNotExist(err) {
		t.Fatal("failed startup retained its socket")
	}
}

package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"yoinkdl/internal/daemon"
	"yoinkdl/internal/queue"
	"yoinkdl/internal/storage"
	"yoinkdl/internal/transfer"
)

func TestDaemonProcessComposedRateLimits(t *testing.T) {
	for _, test := range []struct {
		name    string
		private int64
		shared  int64
	}{
		{"private", 40, 0},
		{"shared", 0, 100},
		{"private slower", 40, 1000},
		{"shared slower", 1000, 100},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := bytes.Repeat([]byte("abcdefgh"), 10)
			var epoch atomic.Pointer[time.Time]
			var began [3]atomic.Pointer[time.Time]
			var calls [3]atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				i, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/"))
				if err != nil || i < 0 || i >= len(began) {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				var start, end int
				n, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end)
				if err != nil || n != 2 || start != 0 || (end != 0 && end != len(data)-1) ||
					(end != 0 && r.Header.Get("If-Range") != `"composed"`) {
					t.Error("limited process lost its probe or conditional range")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				calls[i].Add(1)
				now := time.Now()
				epoch.CompareAndSwap(nil, &now)
				began[i].CompareAndSwap(nil, &now)
				w.Header().Set("ETag", `"composed"`)
				w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
				w.WriteHeader(http.StatusPartialContent)
				w.Write(data[start : end+1])
			}))
			t.Cleanup(srv.Close)

			paths := daemonProcessEnv(t)
			args := []string{"--parallel", "3"}
			if test.shared != 0 {
				args = append(args, "--limit", fmt.Sprintf("%dB/s", test.shared))
			}
			owner := startDaemonProcess(t, args...)
			dir := t.TempDir()
			for i := range began {
				name := strconv.Itoa(i)
				j, err := queue.NewJob(fmt.Sprintf("%032x", i+1), 1, transfer.Request{
					URL: srv.URL + "/" + name, Output: filepath.Join(dir, name), Limit: test.private, Connections: 4,
				})
				if err != nil {
					t.Fatal(err)
				}
				daemonProcessCall(t, paths, "add", map[string]any{"id": j.ID, "request": j.Request}, "")
			}

			tick := time.NewTicker(5 * time.Millisecond)
			defer tick.Stop()
			waitDaemonJobs(t, paths, func(v daemon.List) bool {
				if v.Daemon.LimitBPS != test.shared {
					t.Fatal("daemon lost its startup rate")
				}
				var total int64
				done := 0
				for i, j := range v.Jobs {
					if j.State == queue.Failed {
						t.Fatal("limited process transfer failed")
					}
					if j.Bytes != nil {
						n := *j.Bytes
						if j.State == queue.Completed {
							n++
							done++
						}
						total += n
						assertProcessAllowance(t, test.private, n, began[i].Load())
					}
				}
				assertProcessAllowance(t, test.shared, total, epoch.Load())
				if done == len(began) {
					return true
				}
				<-tick.C
				return false
			})
			daemonProcessCall(t, paths, "stop", map[string]any{}, "")
			waitJSONExit(t, owner.done, 0)
			for i := range began {
				got, err := os.ReadFile(filepath.Join(dir, strconv.Itoa(i)))
				if err != nil || !bytes.Equal(got, data) || calls[i].Load() != 2 {
					t.Fatal("composed process cap changed output or probe/range counts", err)
				}
			}
		})
	}
}

func TestDaemonProcessBudgetSurvivesControlsAndRestart(t *testing.T) {
	const data = "abcdefghijklmnopqrstuvwxyz1234"
	var epoch, resumed atomic.Pointer[time.Time]
	var resumes atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := data[:20]
		if r.URL.Path == "/pause" || r.URL.Path == "/restart" {
			body = data
		}
		now := time.Now()
		epoch.CompareAndSwap(nil, &now)
		w.Header().Set("ETag", `"process-budget"`)
		if r.Header.Get("Range") != "" {
			if r.Header.Get("Range") != "bytes=10-" || r.Header.Get("If-Range") != `"process-budget"` {
				t.Error("limited process resume lost its checkpoint or validator")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if r.URL.Path == "/restart" {
				resumed.Store(&now)
			}
			resumes.Add(1)
			w.Header().Set("Content-Length", strconv.Itoa(len(body)-10))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 10-%d/%d", len(body)-1, len(body)))
			w.WriteHeader(http.StatusPartialContent)
			fmt.Fprint(w, body[10:])
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		switch r.URL.Path {
		case "/pause", "/cancel", "/restart":
			fmt.Fprint(w, body[:10])
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		case "/fail":
			fmt.Fprint(w, body[:10])
		default:
			fmt.Fprint(w, body)
		}
	}))
	t.Cleanup(srv.Close)

	paths := daemonProcessEnv(t)
	owner := startDaemonProcess(t, "--limit", "100B/s")
	dir := t.TempDir()
	add := func(n int, name string, limit int64) string {
		t.Helper()
		j, err := queue.NewJob(fmt.Sprintf("%032x", n), 1, transfer.Request{
			URL: srv.URL + "/" + name + "?token=private-token", Output: filepath.Join(dir, name), Connections: 1, Limit: limit,
		})
		if err != nil {
			t.Fatal(err)
		}
		daemonProcessCall(t, paths, "add", map[string]any{"id": j.ID, "request": j.Request}, "")
		return j.ID
	}
	control := func(op, id string) {
		t.Helper()
		daemonProcessCall(t, paths, op, map[string]any{"id": id}, "")
	}
	seen := make(map[string]int64)
	base := make(map[string]int64)
	var total int64
	var restart string
	rate := int64(100)
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	wait := func(id string, ready func(queue.JobView) bool) {
		t.Helper()
		waitDaemonJobs(t, paths, func(v daemon.List) bool {
			if v.Daemon.LimitBPS != rate {
				t.Fatal("live control replaced the configured shared rate")
			}
			done := false
			for _, j := range v.Jobs {
				if j.Bytes != nil {
					n := max(*j.Bytes-base[j.ID], 0)
					if n > seen[j.ID] {
						total += n - seen[j.ID]
						seen[j.ID] = n
					}
					if j.ID == restart && base[j.ID] > 0 {
						assertProcessAllowance(t, 40, n, resumed.Load())
					}
				}
				if j.ID == id {
					done = ready(j)
				}
			}
			assertProcessAllowance(t, rate, total, epoch.Load())
			if !done {
				<-tick.C
			}
			return done
		})
	}
	ten := func(j queue.JobView) bool { return j.Bytes != nil && *j.Bytes == 10 }
	completed := func(j queue.JobView) bool { return j.State == queue.Completed }
	paused := func(j queue.JobView) bool { return j.State == queue.Paused }

	pause := add(1, "pause", 1000)
	wait(pause, ten)
	control("pause", pause)
	wait(pause, paused)
	daemonProcessCall(t, paths, "set_parallel", map[string]any{"parallel": 3}, "")
	cancel := add(2, "cancel", 1000)
	fail := add(3, "fail", 1000)
	complete := add(4, "complete", 1000)
	control("resume", pause)
	daemonProcessCall(t, paths, "set_parallel", map[string]any{"parallel": 1}, "")
	wait(cancel, ten)
	control("cancel", cancel)
	wait(cancel, func(j queue.JobView) bool { return j.State == queue.Canceled })
	wait(fail, func(j queue.JobView) bool { return j.State == queue.Failed })
	control("resume", fail)
	for _, id := range []string{pause, fail, complete} {
		wait(id, completed)
	}
	last := add(5, "last", 1000)
	wait(last, completed)
	if total != 100 || resumes.Load() != 2 {
		t.Fatal("process lifecycle lost body accounting or repeated a resume")
	}
	restart = add(6, "restart", 40)
	wait(restart, ten)
	control("pause", restart)
	wait(restart, paused)
	if total != 110 {
		t.Fatal("paused restart fixture lost its received prefix")
	}
	daemonProcessCall(t, paths, "stop", map[string]any{}, "")
	waitJSONExit(t, owner.done, 0)
	state := daemonSnapshot(t, paths)
	if state.Parallel != 1 || state.Jobs[5].Request.LimitBPS != 40 || state.Jobs[5].State != queue.Paused {
		t.Fatal("restart lost the private cap or queue settings")
	}

	epoch.Store(nil)
	seen, base = make(map[string]int64), map[string]int64{restart: 10}
	total, rate = 0, 500
	owner = startDaemonProcess(t, "--limit", "500B/s")
	daemonProcessCall(t, paths, "set_parallel", map[string]any{"parallel": 2}, "")
	control("resume", restart)
	after := add(7, "after", 0)
	wait(restart, completed)
	wait(after, completed)
	if total != 40 || resumes.Load() != 3 {
		t.Fatal("new daemon charged saved bytes or repeated network work")
	}
	daemonProcessCall(t, paths, "stop", map[string]any{}, "")
	waitJSONExit(t, owner.done, 0)

	for _, name := range []string{"pause", "cancel", "fail", "complete", "last", "restart", "after"} {
		path, want := filepath.Join(dir, name), data[:20]
		if name == "pause" || name == "restart" {
			want = data
		} else if name == "cancel" {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("canceled limited process published output")
			}
			path += storage.PartSuffix
			want = data[:10]
		}
		if got, err := os.ReadFile(path); err != nil || string(got) != want {
			t.Fatalf("limited process output mismatch for %s: %v", name, err)
		}
	}
}

func assertProcessAllowance(t *testing.T, rate, n int64, start *time.Time) {
	t.Helper()
	if rate == 0 || n == 0 {
		return
	}
	if start == nil {
		t.Fatal("body delivery preceded its HTTP fixture")
	}
	burst := min(int64(65536), (rate-1)/10+1)
	allowed := float64(burst) + float64(rate)*(time.Since(*start)+5*time.Millisecond).Seconds()
	if float64(n) > allowed {
		t.Fatalf("process exceeded %d B/s: delivered=%d, allowance=%f", rate, n, allowed)
	}
}

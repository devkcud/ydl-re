package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"yoinkdl/internal/daemon"
	"yoinkdl/internal/queue"
	"yoinkdl/internal/storage"
	"yoinkdl/internal/transfer"
)

func TestDaemonProcessCapacityAndForgetRecovery(t *testing.T) {
	const data = "abcdefghijklmnopqrst"
	var completeCalls, partialCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/complete":
			completeCalls.Add(1)
			fmt.Fprint(w, data)
		case "/partial":
			w.Header().Set("ETag", `"capacity"`)
			if partialCalls.Add(1) == 1 {
				w.Header().Set("Content-Length", "20")
				fmt.Fprint(w, data[:5])
				w.(http.Flusher).Flush()
				<-r.Context().Done()
				return
			}
			if r.Header.Get("Range") != "bytes=5-" || r.Header.Get("If-Range") != `"capacity"` {
				t.Error("readmitted partial lost its checkpoint or validator")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Length", "15")
			w.Header().Set("Content-Range", "bytes 5-19/20")
			w.WriteHeader(http.StatusPartialContent)
			fmt.Fprint(w, data[5:])
		default:
			t.Error("a waiting, canceled, or refused job reached the server")
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	paths := daemonProcessEnv(t)
	owner := startDaemonProcess(t)
	dir := t.TempDir()
	newJob := func(n int, name string) queue.Job {
		t.Helper()
		j, err := queue.NewJob(fmt.Sprintf("%032x", n+1), 1, transfer.Request{
			URL: srv.URL + "/" + name + "?token=private-token", Output: filepath.Join(dir, name),
			Connections: 1, Timeout: 2 * time.Minute,
		})
		if err != nil {
			t.Fatal(err)
		}
		return j
	}
	add := func(j queue.Job, code string) queue.JobView {
		t.Helper()
		r := daemonProcessCall(t, paths, "add", map[string]any{"id": j.ID, "request": j.Request}, code)
		var view queue.JobView
		if code == "" {
			if err := json.Unmarshal(r.Result, &view); err != nil || view.ID != j.ID {
				t.Fatal("add lost the submitted job identity", err)
			}
		}
		return view
	}
	forget := func(id string, want bool) {
		t.Helper()
		r := daemonProcessCall(t, paths, "forget", map[string]any{"id": id}, "")
		var result daemon.Forgotten
		if err := json.Unmarshal(r.Result, &result); err != nil || result.ID != id || result.Forgotten != want {
			t.Fatalf("forget = %+v, %v", result, err)
		}
	}
	read := func(path string) []byte {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	unchanged := func(path string, want []byte) {
		t.Helper()
		if !bytes.Equal(read(path), want) {
			t.Fatalf("unexpected change to %s", path)
		}
	}
	stop := func(kill bool) {
		t.Helper()
		code := 0
		if kill {
			if err := owner.cmd.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			code = -1
		} else {
			daemonProcessCall(t, paths, "stop", map[string]any{}, "")
		}
		waitJSONExit(t, owner.done, code)
		if owner.err.Len() != 0 {
			t.Fatalf("owner stderr = %q", &owner.err)
		}
	}
	jobs := make([]queue.Job, queue.MaxJobs)
	jobs[0], jobs[1] = newJob(0, "complete"), newJob(1, "partial")
	add(jobs[0], "")
	waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Jobs[0].State == queue.Completed })
	add(jobs[1], "")
	waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Jobs[1].Bytes != nil && *v.Jobs[1].Bytes == 5 })
	for i := 2; i < len(jobs); i++ {
		jobs[i] = newJob(i, fmt.Sprintf("waiting-%03d", i))
		if got := add(jobs[i], ""); got.Order != int64(i+1) || got.State != queue.Queued {
			t.Fatal("full-queue setup lost admission order or the occupied slot")
		}
		daemonProcessCall(t, paths, "cancel", map[string]any{"id": jobs[i].ID}, "")
	}
	extra := newJob(queue.MaxJobs, "overflow")
	add(extra, "capacity")
	daemonProcessCall(t, paths, "forget", map[string]any{"id": jobs[1].ID}, "busy")
	daemonProcessCall(t, paths, "cancel", map[string]any{"id": jobs[1].ID}, "")
	view := waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Daemon.Active == 0 })
	if len(view.Jobs) != queue.MaxJobs {
		t.Fatal("terminal transitions freed retained capacity")
	}
	for i, j := range view.Jobs {
		want := queue.Canceled
		if i == 0 {
			want = queue.Completed
		}
		if j.ID != jobs[i].ID || j.Order != int64(i+1) || j.State != want {
			t.Fatal("full queue lost a retained terminal record")
		}
	}
	final := jobs[0].Request.Output
	part := jobs[1].Request.Output + storage.PartSuffix
	manifest := jobs[1].Request.Output + storage.ManifestSuffix
	meta := read(manifest)
	file, err := os.Stat(final)
	if err != nil {
		t.Fatal(err)
	}
	unchanged(final, []byte(data))
	unchanged(part, []byte(data[:5]))
	m, err := storage.Inspect(jobs[1].Request.Output)
	if err != nil || m.Completed != 5 || m.State != storage.StatePartial {
		t.Fatalf("retained partial checkpoint = %+v, %v", m, err)
	}
	snapshot := filepath.Join(paths.State, queue.SnapshotName)
	full := read(snapshot)
	for i := range 2 {
		if got := add(jobs[i], ""); got.Order != view.Jobs[i].Order || got.State != view.Jobs[i].State {
			t.Fatal("full queue did not deduplicate its retained terminal job")
		}
	}
	forget(extra.ID, false)
	add(extra, "capacity")
	blocked := extra
	blocked.Request.Output = final
	add(blocked, "conflict")
	unchanged(snapshot, full)
	stop(true)
	state := daemonSnapshot(t, paths)
	if len(state.Jobs) != queue.MaxJobs || state.NextOrder != queue.MaxJobs+1 {
		t.Fatal("forced death lost full retention or consumed refused admission orders")
	}
	for _, j := range state.Jobs {
		if j.Request.URL != nil {
			t.Fatal("full terminal queue retained a source")
		}
	}
	owner = startDaemonProcess(t)
	unchanged(snapshot, full)
	add(extra, "capacity")
	for i := range 2 {
		if got := add(jobs[i], ""); got.Order != view.Jobs[i].Order || got.State != view.Jobs[i].State {
			t.Fatal("restart lost terminal add deduplication")
		}
	}
	checkCapacityCLI(t, *extra.Request.URL, extra.Request.Output)
	unchanged(snapshot, full)
	for _, suffix := range []string{"", storage.PartSuffix, storage.ManifestSuffix, storage.LockSuffix} {
		if _, err := os.Stat(extra.Request.Output + suffix); !os.IsNotExist(err) {
			t.Fatal("refused admission created an output artifact", err)
		}
	}
	forget(jobs[0].ID, true)
	freed := read(snapshot)
	forget(jobs[0].ID, false)
	unchanged(snapshot, freed)
	unchanged(final, []byte(data))
	stop(true)
	state = daemonSnapshot(t, paths)
	if len(state.Jobs) != queue.MaxJobs-1 || state.NextOrder != queue.MaxJobs+1 || state.Jobs[0].ID != jobs[1].ID {
		t.Fatal("forget did not durably release one slot without consuming an order")
	}
	owner = startDaemonProcess(t)
	unchanged(snapshot, freed)
	if got := add(jobs[0], ""); got.Order != state.NextOrder {
		t.Fatal("forgotten ID reused its old admission or retained its output reservation")
	}
	view = waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Jobs[len(v.Jobs)-1].State == queue.Failed })
	last := view.Jobs[len(view.Jobs)-1]
	if len(view.Jobs) != queue.MaxJobs || last.ID != jobs[0].ID || last.Published == nil || *last.Published || completeCalls.Load() != 1 {
		t.Fatal("readmission inferred success or fetched a preexisting final output")
	}
	full = read(snapshot)
	add(extra, "capacity")
	unchanged(snapshot, full)
	unchanged(final, []byte(data))
	forget(jobs[1].ID, true)
	freed = read(snapshot)
	unchanged(part, []byte(data[:5]))
	unchanged(manifest, meta)
	stop(false)
	owner = startDaemonProcess(t)
	unchanged(snapshot, freed)
	if got := add(jobs[1], ""); got.Order != queue.MaxJobs+2 {
		t.Fatal("clean restart lost the freed partial output reservation or admission order")
	}
	view = waitDaemonJobs(t, paths, func(v daemon.List) bool { return v.Jobs[len(v.Jobs)-1].State == queue.Completed })
	if len(view.Jobs) != queue.MaxJobs || view.Jobs[len(view.Jobs)-1].ID != jobs[1].ID || partialCalls.Load() != 2 {
		t.Fatal("forgotten partial did not resume exactly once into the released slot")
	}
	full = read(snapshot)
	add(extra, "capacity")
	unchanged(snapshot, full)
	stop(false)
	state = daemonSnapshot(t, paths)
	if len(state.Jobs) != queue.MaxJobs || state.NextOrder != queue.MaxJobs+3 || state.Jobs[len(state.Jobs)-1].Request.URL != nil {
		t.Fatal("refilled queue lost retention, order, or terminal source cleanup")
	}
	unchanged(final, []byte(data))
	info, err := os.Stat(final)
	if err != nil || !os.SameFile(file, info) {
		t.Fatal("forget or readmission replaced the completed file", err)
	}
	unchanged(jobs[1].Request.Output, []byte(data))
	for _, path := range []string{part, manifest} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("resumed completion retained a sidecar", err)
		}
	}
}

func daemonProcessCall(t *testing.T, paths daemon.Paths, op string, args any, code string) daemon.Reply {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, err := daemon.Call(ctx, paths, op, args)
	if code == "" {
		if err != nil || !r.OK || r.Error != nil {
			t.Fatalf("%s failed: %v", op, err)
		}
	} else if err == nil || r.OK || r.Error == nil || r.Error.Code != code {
		t.Fatalf("%s refusal = %v, want %s", op, err, code)
	}
	if bytes.Contains(r.Result, []byte("private-token")) || err != nil && strings.Contains(err.Error(), "private-token") {
		t.Fatal("daemon exposed a private source")
	}
	return r
}

func checkCapacityCLI(t *testing.T, url, output string) {
	t.Helper()
	cmd := command("add", url, "-o", output, "-c", "1", "--json")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || stderr.Len() != 0 {
		t.Fatalf("full-queue CLI exit = %v, stderr=%q", err, &stderr)
	}
	var result struct {
		Version int             `json:"schema_version"`
		Command string          `json:"command"`
		ID      string          `json:"job_id"`
		Code    int             `json:"exit_code"`
		Error   *daemon.Failure `json:"error"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || result.Version != 1 || result.Command != "add" ||
		len(result.ID) != 32 || result.Code != 1 || result.Error == nil || result.Error.Code != "capacity" || bytes.Contains(raw, []byte("private-token")) {
		t.Fatalf("full-queue CLI result = %+v, %v", result, err)
	}
}

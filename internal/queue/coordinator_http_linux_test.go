package queue

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"yoinkdl/internal/transfer"
)

func TestQueueRealTransferPauseResumeAndNoOverwrite(t *testing.T) {
	data := bytes.Repeat([]byte("queue fixture\n"), 16*1024)
	const prefix = 8192
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requests.Add(1)
		w.Header().Set("ETag", `"queue-fixture"`)
		if n == 1 {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.Write(data[:prefix])
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		if n != 2 || r.Header.Get("Range") != fmt.Sprintf("bytes=%d-", prefix) || r.Header.Get("If-Range") != `"queue-fixture"` {
			t.Error("queue resume bypassed the engine's conditional prefix validation")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)-prefix))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", prefix, len(data)-1, len(data)))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(data[prefix:])
	}))
	t.Cleanup(server.Close)
	dir := privateDir(t)
	s := testStore(t, dir)
	c, err := NewCoordinator(s, nil, 0)
	must(t, err)
	done := runCoordinator(t, c)
	req := transfer.Request{URL: server.URL + "/payload?token=queue-private", Output: filepath.Join(dir, "download"), Connections: 1}
	id := fmt.Sprintf("%032x", 1)
	_, err = c.Add(testContext(t), id, req)
	must(t, err)
	waitQueue(t, c, func(v ListView) bool { return v.Jobs[0].Bytes != nil && *v.Jobs[0].Bytes == prefix })
	_, err = c.Pause(testContext(t), id)
	must(t, err)
	waitQueue(t, c, func(v ListView) bool { return v.Jobs[0].State == Paused })
	if !bytes.Equal(readFile(t, req.Output+".yoinkdl.part"), data[:prefix]) {
		t.Fatal("pause did not preserve the verified prefix")
	}
	if _, err := os.Stat(req.Output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("paused transfer was published")
	}
	if !bytes.Contains(readFile(t, filepath.Join(dir, SnapshotName)), []byte("queue-private")) {
		t.Fatal("paused job lost the source required for resume")
	}
	_, err = c.Resume(testContext(t), id)
	must(t, err)
	waitQueue(t, c, func(v ListView) bool { return v.Jobs[0].State == Completed })
	if requests.Load() != 2 || !bytes.Equal(readFile(t, req.Output), data) {
		t.Fatal("resumed queue transfer did not publish the exact fixture")
	}
	if bytes.Contains(readFile(t, filepath.Join(dir, SnapshotName)), []byte("queue-private")) {
		t.Fatal("completed queue retained its source token")
	}

	_, err = c.Forget(testContext(t), id)
	must(t, err)
	_, err = c.Add(testContext(t), fmt.Sprintf("%032x", 2), req)
	must(t, err)
	view := waitQueue(t, c, func(v ListView) bool { return v.Jobs[0].State == Failed })
	if view.Jobs[0].Published == nil || *view.Jobs[0].Published || requests.Load() != 2 || !bytes.Equal(readFile(t, req.Output), data) {
		t.Fatal("retry over a completed output bypassed no-overwrite protection")
	}
	_, err = c.Stop(testContext(t))
	must(t, err)
	must(t, waitRun(t, done))
}

package transfer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSharedBudgetDownload(t *testing.T) {
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
			const data = "abcdefghijklmnopqrst"
			var traffic atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				start, end := requestedRange(t, r, int64(len(data)))
				w.Header().Set("ETag", `"v1"`)
				w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
				w.WriteHeader(http.StatusPartialContent)
				n, _ := fmt.Fprint(w, data[start:end+1])
				traffic.Add(int64(n))
			}))
			defer server.Close()
			dir := t.TempDir()
			start := time.Now()
			budget, err := NewBudget(test.shared)
			if err != nil {
				t.Fatal(err)
			}
			var mu sync.Mutex
			var total int64
			var group sync.WaitGroup
			for i := range 4 {
				group.Go(func() {
					path := filepath.Join(dir, strconv.Itoa(i))
					began := time.Now()
					var last int64
					result, err := budget.Download(context.Background(), Request{URL: server.URL, Output: path, Limit: test.private}, func(s Snapshot) {
						mu.Lock()
						defer mu.Unlock()
						total += s.Bytes - last
						last = s.Bytes
						assertHTTPAllowance(t, test.shared, total, start)
						assertHTTPAllowance(t, test.private, s.Bytes, began)
					})
					if err != nil || !result.Published || result.Bytes != int64(len(data)) {
						t.Errorf("shared download = %+v, %v", result, err)
						return
					}
					assertHTTPAllowance(t, test.private, int64(len(data)+1), began)
					assertData(t, path, data)
				})
			}
			group.Wait()
			if traffic.Load() != 84 || total != 80 {
				t.Fatalf("traffic=%d, useful=%d; want 84, 80", traffic.Load(), total)
			}
			assertHTTPAllowance(t, test.shared, traffic.Load(), start)
		})
	}
}

func TestZeroBudgetDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "abcd")
	}))
	defer server.Close()
	var budget Budget
	path := filepath.Join(t.TempDir(), "file")
	start := time.Now()
	result, err := budget.Download(context.Background(), Request{URL: server.URL, Output: path, Connections: 1, Limit: 10}, nil)
	if err != nil || !result.Published {
		t.Fatalf("zero budget download = %+v, %v", result, err)
	}
	assertData(t, path, "abcd")
	assertHTTPAllowance(t, 10, 4, start)
}

func assertHTTPAllowance(t *testing.T, rate, n int64, start time.Time) {
	t.Helper()
	if rate == 0 {
		return
	}
	burst := min(int64(bufferSize), (rate-1)/10+1)
	allowed := float64(burst) + float64(rate)*(time.Since(start)+5*time.Millisecond).Seconds()
	if float64(n) > allowed {
		t.Errorf("delivered %d bytes at %d B/s; allowance %f", n, rate, allowed)
	}
}

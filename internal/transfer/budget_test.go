package transfer

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestSharedBudgetAcrossIndependentClients(t *testing.T) {
	for _, test := range []struct {
		name    string
		private int64
		shared  int64
	}{
		{"unlimited", 0, 0},
		{"private", 1000, 0},
		{"shared", 0, 1000},
		{"both", 1000, 1000},
		{"private slower", 10, 1000},
		{"shared slower", 1000, 10},
		{"minimum", 1, 1},
		{"rounded burst", 11, 11},
		{"maximum", math.MaxInt64, math.MaxInt64},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				shared, err := NewBudget(test.shared)
				if err != nil {
					t.Fatal(err)
				}
				start := time.Now()
				var mu sync.Mutex
				var total int
				var group sync.WaitGroup
				for range 4 {
					var delivered int
					client := rateClient(t, test.private, time.Millisecond, func(req *http.Request) (*http.Response, error) {
						reader := strings.NewReader(strings.Repeat("x", 40))
						return &http.Response{StatusCode: http.StatusOK, Request: req, Header: make(http.Header),
							Body: io.NopCloser(readFunc(func(p []byte) (int, error) {
								for _, rate := range []int64{test.private, test.shared} {
									if rate > 0 && int64(len(p)) > min(int64(bufferSize), (rate-1)/10+1) {
										t.Error("read exceeded the smallest burst")
									}
								}
								return reader.Read(p)
							}))}, nil
					}, shared)
					for range 4 {
						group.Go(func() {
							resp, err := client.Get("http://example.org/file")
							if err != nil {
								t.Error(err)
								return
							}
							defer resp.Body.Close()
							buf := make([]byte, bufferSize)
							for {
								n, err := resp.Body.Read(buf)
								mu.Lock()
								total += n
								delivered += n
								assertBudgetAllowance(t, test.private, delivered, start)
								assertBudgetAllowance(t, test.shared, total, start)
								mu.Unlock()
								if err != nil {
									if err != io.EOF {
										t.Error(err)
									}
									return
								}
							}
						})
					}
				}
				group.Wait()
				if total != 640 {
					t.Fatalf("delivered %d bytes; want 640", total)
				}
			})
		})
	}
}

func TestBudgetCompositionWaitsForLatestReservation(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			slow, _ := NewBudget(1)
			fast, _ := NewBudget(2)
			budgets := []*Budget{slow, fast}
			if reverse {
				budgets = []*Budget{fast, slow}
			}
			if err := waitBudgets(context.Background(), 1, budgets...); err != nil {
				t.Fatal(err)
			}
			start := time.Now()
			if err := waitBudgets(context.Background(), 1, budgets...); err != nil {
				t.Fatal(err)
			}
			if time.Since(start) != time.Second {
				t.Fatalf("combined waits took %s; want 1s", time.Since(start))
			}
		})
	}
}

func TestSharedBudgetChargesEveryReadBeforeCancellation(t *testing.T) {
	for _, when := range []string{"read", "wait", "error"} {
		t.Run(when, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				private, _ := NewBudget(1)
				shared, _ := NewBudget(1)
				if err := waitBudgets(context.Background(), 1, private, shared); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancelCause(context.Background())
				defer cancel(nil)
				body := &responseBody{ctx: ctx, cancel: cancel, timeout: time.Millisecond, budgets: []*Budget{private, shared},
					ReadCloser: io.NopCloser(readFunc(func(p []byte) (int, error) {
						if when == "read" {
							cancel(context.Canceled)
						}
						return copy(p, "x"), io.ErrUnexpectedEOF
					}))}
				defer body.Close()
				done := make(chan error, 1)
				go func() {
					n, err := body.Read(make([]byte, 10))
					if when == "error" && n != 1 || when != "error" && n != 0 {
						t.Errorf("read delivered %d bytes at %s", n, when)
					}
					done <- err
				}()
				synctest.Wait()
				if when == "wait" {
					cancel(context.Canceled)
				}
				want := error(context.Canceled)
				if when == "error" {
					want = io.ErrUnexpectedEOF
				}
				if err := <-done; !errors.Is(err, want) {
					t.Fatalf("read error = %v; want %v", err, want)
				}
				start := time.Now()
				var group sync.WaitGroup
				for _, budget := range []*Budget{private, shared} {
					group.Go(func() {
						if err := waitBudgets(context.Background(), 1, budget); err != nil {
							t.Error(err)
						}
						wait := 2 * time.Second
						if when == "error" {
							wait = time.Second
						}
						if time.Since(start) != wait {
							t.Errorf("already-read bytes lost their charge: elapsed=%s, want=%s", time.Since(start), wait)
						}
					})
				}
				group.Wait()
			})
		})
	}
}

func TestSharedBudgetSurvivesNewClientsAndIdle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		shared, _ := NewBudget(10)
		for round := range 2 {
			start := time.Now()
			for range 3 {
				client := rateClient(t, 10, time.Millisecond, func(req *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: http.StatusOK, Request: req, Header: make(http.Header),
						Body: io.NopCloser(strings.NewReader("ab"))}, nil
				}, shared)
				resp, err := client.Get("http://example.org/file")
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil || string(data) != "ab" {
					t.Fatalf("body = %q, %v", data, err)
				}
			}
			if time.Since(start) != 500*time.Millisecond {
				t.Fatalf("round %d minted credit: elapsed=%s", round, time.Since(start))
			}
			time.Sleep(time.Hour)
		}
	})
}

func assertBudgetAllowance(t *testing.T, rate int64, n int, start time.Time) {
	t.Helper()
	if rate == 0 {
		return
	}
	burst := min(int64(bufferSize), (rate-1)/10+1)
	allowed := float64(burst) + float64(rate)*(time.Since(start)+time.Microsecond).Seconds()
	if float64(n) > allowed {
		t.Errorf("delivered %d bytes at %d B/s; allowance %f", n, rate, allowed)
	}
}

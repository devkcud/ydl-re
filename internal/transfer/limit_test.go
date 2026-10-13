package transfer

import (
	"context"
	"errors"
	"math"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestParseLimit(t *testing.T) {
	for value, want := range map[string]int64{
		"1B/s": 1, "0010B/s": 10, "5KB/s": 5000, "5MB/s": 5000000,
		"5GB/s": 5000000000, "5KiB/s": 5120, "5MiB/s": 5242880, "5GiB/s": 5368709120,
		"9223372036854775807B/s": math.MaxInt64, "9223372036854775KB/s": 9223372036854775000,
		"8589934591GiB/s": 9223372035781033984,
	} {
		t.Run(value, func(t *testing.T) {
			got, err := ParseLimit(value)
			if err != nil || got != want {
				t.Fatalf("rate = %d, %v; want %d", got, err, want)
			}
		})
	}

	for _, value := range []string{
		"", "0", "0B/s", "000GB/s", "B/s", "1", "1MB", "1/s", "1TB/s", "1Mb/s", "1mb/s",
		"1MB/S", "1MB/sec", "+1B/s", "-1B/s", "1.5MB/s", "1.0B/s", "1e3B/s", "0x10B/s",
		" 1B/s", "1B/s ", "1 B/s", "1\tB/s", "１B/s", "1B/s\n", "1_000B/s", "unlimited",
		"9223372036854775808B/s", "9223372036854776KB/s", "8589934592GiB/s", "99999999999999999999999MB/s",
	} {
		t.Run(value, func(t *testing.T) {
			if n, err := ParseLimit(value); err == nil || n != 0 {
				t.Fatalf("invalid rate accepted: %d, %v", n, err)
			}
		})
	}
}

func TestLimiterAggregateBudget(t *testing.T) {
	for _, workers := range []int{1, 4, MaxConnections} {
		t.Run(strconv.Itoa(workers), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				l, err := NewBudget(1000)
				if err != nil {
					t.Fatal(err)
				}
				start := time.Now()
				var mu sync.Mutex
				var total int
				var group sync.WaitGroup
				for range workers {
					group.Go(func() {
						for range 5 {
							if err := waitBudgets(context.Background(), 100, l); err != nil {
								t.Error(err)
								return
							}
							mu.Lock()
							total += 100
							allowed := 100 + 1000*time.Since(start).Seconds()
							if float64(total) > allowed+0.00001 {
								t.Errorf("delivered %d bytes; allowance %f", total, allowed)
							}
							mu.Unlock()
						}
					})
				}
				group.Wait()
				want := time.Duration(workers*5-1) * 100 * time.Millisecond
				if total != workers*500 || time.Since(start) < want || time.Since(start) > want+time.Microsecond {
					t.Fatalf("%d workers delivered %d bytes in %s; want %s", workers, total, time.Since(start), want)
				}
			})
		})
	}
}

func TestLimiterIdleCreditIsBounded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l, err := NewBudget(1000)
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			start := time.Now()
			for range 2 {
				if err := waitBudgets(context.Background(), 100, l); err != nil {
					t.Fatal(err)
				}
			}
			if elapsed := time.Since(start); elapsed < 100*time.Millisecond || elapsed > 100*time.Millisecond+time.Nanosecond {
				t.Fatalf("burst after idle took %s", elapsed)
			}
			time.Sleep(10 * time.Second)
		}
	})
}

func TestLimiterCanceledReadsKeepTheirCharge(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l, err := NewBudget(100)
		if err != nil {
			t.Fatal(err)
		}
		if err := waitBudgets(context.Background(), 10, l); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- waitBudgets(ctx, 10, l) }()
		synctest.Wait()
		select {
		case err := <-done:
			t.Fatalf("exhausted budget did not wait: %v", err)
		default:
		}
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation = %v", err)
		}
		start := time.Now()
		if err := waitBudgets(context.Background(), 10, l); err != nil {
			t.Fatal(err)
		}
		if elapsed := time.Since(start); elapsed < 200*time.Millisecond || elapsed > 200*time.Millisecond+time.Nanosecond {
			t.Fatalf("canceled attempt's bytes were refunded: waited %s", elapsed)
		}
	})
}

func TestLimiterRateBounds(t *testing.T) {
	for _, rate := range []int64{0, 1, 11, math.MaxInt64} {
		synctest.Test(t, func(t *testing.T) {
			l, err := NewBudget(rate)
			if err != nil {
				t.Fatal(err)
			}
			if rate == 0 {
				if l != nil || waitBudgets(context.Background(), bufferSize, l) != nil {
					t.Fatal("zero did not disable the budget")
				}
				return
			}
			burst := min(int64(bufferSize), (rate-1)/10+1)
			if l.burst != int(burst) {
				t.Fatalf("burst = %d; want %d", l.burst, burst)
			}
			start := time.Now()
			for range 2 {
				if err := waitBudgets(context.Background(), l.burst, l); err != nil {
					t.Fatal(err)
				}
			}
			want := time.Duration(math.Ceil(float64(burst) / float64(rate) * float64(time.Second)))
			if elapsed := time.Since(start); elapsed != want {
				t.Fatalf("rate %d waited %s; want %s", rate, elapsed, want)
			}
		})
	}
	if _, err := NewBudget(-1); err == nil {
		t.Fatal("negative rate accepted")
	}
}

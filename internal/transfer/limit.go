package transfer

import (
	"context"
	"errors"
	"math"
	"strconv"
	"sync"
	"time"
)

type Budget struct {
	mu     sync.Mutex
	rate   float64
	burst  int
	tokens float64
	last   time.Time
}

func ParseLimit(value string) (int64, error) {
	i := 0
	for i < len(value) && value[i] >= '0' && value[i] <= '9' {
		i++
	}

	var unit int64
	switch value[i:] {
	case "B/s":
		unit = 1
	case "KB/s":
		unit = 1000
	case "MB/s":
		unit = 1000 * 1000
	case "GB/s":
		unit = 1000 * 1000 * 1000
	case "KiB/s":
		unit = 1024
	case "MiB/s":
		unit = 1024 * 1024
	case "GiB/s":
		unit = 1024 * 1024 * 1024
	}

	n, err := strconv.ParseInt(value[:i], 10, 64)
	if err != nil || n <= 0 || unit == 0 || n > math.MaxInt64/unit {
		return 0, errors.New("limit must be a positive integer followed by B/s, KB/s, MB/s, GB/s, KiB/s, MiB/s, or GiB/s, within 9223372036854775807 bytes/s")
	}

	return n * unit, nil
}

func NewBudget(rate int64) (*Budget, error) {
	if rate < 0 {
		return nil, errors.New("limit must not be negative")
	}
	if rate == 0 {
		return nil, nil
	}

	burst := min(int64(bufferSize), (rate-1)/10+1)
	return &Budget{rate: float64(rate), burst: int(burst), tokens: float64(burst), last: time.Now()}, nil
}

func (b *Budget) reserve(n int) (time.Time, error) {
	if b == nil || b.rate == 0 || n == 0 {
		return time.Time{}, nil
	}
	if n < 0 || n > b.burst {
		return time.Time{}, errors.New("body read exceeds the rate budget's burst")
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.tokens = min(float64(b.burst), b.tokens+now.Sub(b.last).Seconds()*b.rate) - float64(n)
	b.last = now
	delay := time.Duration(math.Ceil(max(0, -b.tokens) / b.rate * float64(time.Second)))
	return now.Add(delay), nil
}

func waitBudgets(ctx context.Context, n int, budgets ...*Budget) error {
	var ready time.Time
	for _, b := range budgets {
		at, err := b.reserve(n)
		if err != nil {
			return err
		}
		if at.After(ready) {
			ready = at
		}
	}

	delay := time.Until(ready)
	if cause := context.Cause(ctx); cause != nil || delay <= 0 {
		return cause
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-timer.C:
		return context.Cause(ctx)
	}
}

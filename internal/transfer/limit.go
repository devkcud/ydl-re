package transfer

import (
	"context"
	"errors"
	"math"
	"strconv"
	"sync"
	"time"
)

type limiter struct {
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

func newLimiter(rate int64) (*limiter, error) {
	if rate < 0 {
		return nil, errors.New("limit must not be negative")
	}
	if rate == 0 {
		return nil, nil
	}

	burst := min(int64(bufferSize), (rate-1)/10+1)
	return &limiter{rate: float64(rate), burst: int(burst), tokens: float64(burst), last: time.Now()}, nil
}

func (l *limiter) wait(ctx context.Context, n int) error {
	if l == nil || n == 0 {
		return context.Cause(ctx)
	}
	if n < 0 || n > l.burst {
		return errors.New("body read exceeds the rate budget's burst")
	}

	l.mu.Lock()
	now := time.Now()
	l.tokens = min(float64(l.burst), l.tokens+now.Sub(l.last).Seconds()*l.rate) - float64(n)
	l.last = now
	delay := time.Duration(math.Ceil(max(0, -l.tokens) / l.rate * float64(time.Second)))
	l.mu.Unlock()

	if cause := context.Cause(ctx); cause != nil || delay == 0 {
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

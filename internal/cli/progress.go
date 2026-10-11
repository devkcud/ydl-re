package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"yoinkdl/internal/transfer"
)

const progressInterval = 200 * time.Millisecond

type progress struct {
	out      *recordWriter
	err      error
	start    time.Time
	last     time.Time
	latest   transfer.Snapshot
	started  bool
	finished bool
	inline   bool
	terminal *os.File
}

func newProgress(out io.Writer) (*progress, error) {
	w, err := newRecordWriter(out)
	if err != nil {
		return nil, err
	}
	p := &progress{out: w, start: time.Now()}

	if file, ok := w.out.(*os.File); ok && os.Getenv("TERM") != "" && os.Getenv("TERM") != "dumb" {
		_, p.inline = terminalSize(file)
		if p.inline {
			p.terminal = file
		}
	}

	return p, nil
}

func (p *progress) Update(ctx context.Context, s transfer.Snapshot) error {
	if p.err != nil || p.finished {
		return p.err
	}
	if !p.started {
		p.start = time.Now()
	}

	p.latest = s
	p.started = true

	if time.Since(p.last) >= progressInterval {
		return p.render(ctx, false)
	}
	return nil
}

func (p *progress) Finish() error {
	if p.finished {
		return p.err
	}
	p.finished = true

	p.latest.Workers = 0
	if p.started && p.err == nil {
		p.render(context.Background(), true)
	}
	if err := p.out.Close(); err != nil {
		p.err = errors.Join(p.err, fmt.Errorf("close progress output: %w", err))
	}
	return p.err
}

func (p *progress) render(ctx context.Context, final bool) error {
	s := p.latest
	rate := 0.0
	if elapsed := time.Since(p.start).Seconds(); elapsed > 0 {
		rate = float64(s.Bytes-s.Resumed) / elapsed
	}
	workers := fmt.Sprintf("%d workers", s.Workers)
	if s.Workers == 1 {
		workers = "1 worker"
	}
	line := fmt.Sprintf("%s | %s/s | %s", size(float64(s.Bytes)), size(rate), workers)

	if s.Total >= 0 {
		percent := 100.0
		eta := "--"

		if s.Total > 0 {
			percent = 100 * float64(s.Bytes) / float64(s.Total)
		}

		if rate > 0 {
			seconds := float64(s.Total-s.Bytes) / rate
			eta = fmt.Sprintf("%.0fs", seconds)
		}

		line = fmt.Sprintf("%s / %s (%.1f%%) | %s/s | ETA %s | %s",
			size(float64(s.Bytes)), size(float64(s.Total)), percent, size(rate), eta, workers)
	}

	if p.inline {
		if width, ok := terminalSize(p.terminal); ok && width > 0 && len(line) >= width {
			line = line[:width-1]
			if len(line) > 3 {
				line = line[:len(line)-3] + "..."
			}
		}
		line = "\r\x1b[2K" + line
	}
	if !p.inline || final {
		line += "\n"
	}
	n, err := p.out.Write(ctx, []byte(line))
	if err != nil {
		err = fmt.Errorf("write progress: %w", err)
		if n != 0 || ctx.Err() == nil {
			p.err = err
		}
	} else {
		p.last = time.Now()
	}
	return err
}

func size(value float64) string {
	const unit = 1024
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	index := 0

	for value >= unit && index < len(units)-1 {
		value /= unit
		index++
	}

	return fmt.Sprintf("%.1f %s", value, units[index])
}

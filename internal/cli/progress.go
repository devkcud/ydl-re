package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	"yoinkdl/internal/transfer"
)

const progressInterval = 200 * time.Millisecond

type progress struct {
	out     io.Writer
	start   time.Time
	last    time.Time
	latest  transfer.Snapshot
	started bool
	inline  bool
}

func newProgress(out io.Writer) *progress {
	p := &progress{out: out, start: time.Now()}

	if file, ok := out.(*os.File); ok {
		info, err := file.Stat()
		p.inline = err == nil && info.Mode()&os.ModeCharDevice != 0
	}

	return p
}

func (p *progress) Update(s transfer.Snapshot) {
	if !p.started {
		p.start = time.Now()
	}

	p.latest = s
	p.started = true

	if time.Since(p.last) >= progressInterval {
		p.render()
	}
}

func (p *progress) Finish() {
	if !p.started {
		return
	}

	p.render()

	if p.inline {
		fmt.Fprintln(p.out)
	}
}

func (p *progress) render() {
	p.last = time.Now()
	s := p.latest
	rate := 0.0
	if elapsed := time.Since(p.start).Seconds(); elapsed > 0 {
		rate = float64(s.Bytes-s.Resumed) / elapsed
	}
	line := fmt.Sprintf("%s | %s/s | 1 worker", size(float64(s.Bytes)), size(rate))

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

		line = fmt.Sprintf("%s / %s (%.1f%%) | %s/s | ETA %s | 1 worker",
			size(float64(s.Bytes)), size(float64(s.Total)), percent, size(rate), eta)
	}

	if p.inline {
		fmt.Fprint(p.out, "\r\x1b[2K"+line)
	} else {
		fmt.Fprintln(p.out, line)
	}
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

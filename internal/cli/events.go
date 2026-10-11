package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"yoinkdl/internal/transfer"
)

const eventVersion = 1

type progressEvent struct {
	Version int    `json:"schema_version"`
	Event   string `json:"event"`
	Elapsed int64  `json:"elapsed_ms"`
	Bytes   int64  `json:"bytes"`
	Total   *int64 `json:"total"`
	Resumed int64  `json:"resumed"`
	Workers int    `json:"workers"`
}

type resultEvent struct {
	Version   int     `json:"schema_version"`
	Event     string  `json:"event"`
	Elapsed   int64   `json:"elapsed_ms"`
	Status    string  `json:"status"`
	ExitCode  int     `json:"exit_code"`
	Path      *string `json:"path"`
	Partial   *string `json:"partial"`
	Bytes     int64   `json:"bytes"`
	Total     *int64  `json:"total"`
	Resumed   int64   `json:"resumed"`
	Published bool    `json:"published"`
	Error     *string `json:"error"`
}

type events struct {
	out      *recordWriter
	err      error
	quiet    bool
	finished bool
	start    time.Time
	last     time.Time
	latest   transfer.Snapshot
}

func newEvents(out io.Writer, quiet bool) (*events, error) {
	w, err := newRecordWriter(out)
	if err != nil {
		return nil, err
	}

	return &events{out: w, quiet: quiet, start: time.Now(), latest: transfer.Snapshot{Total: -1}}, nil
}

func (e *events) Update(ctx context.Context, s transfer.Snapshot) error {
	if e.err != nil || e.finished {
		return e.err
	}

	e.latest = s
	if e.quiet || ctx.Err() != nil || (!e.last.IsZero() && time.Since(e.last) < progressInterval) {
		return nil
	}

	err := e.write(ctx, progressEvent{
		Version: eventVersion, Event: "progress", Elapsed: time.Since(e.start).Milliseconds(),
		Bytes: s.Bytes, Total: knownTotal(s.Total), Resumed: s.Resumed, Workers: s.Workers,
	})
	if err == nil {
		e.last = time.Now()
	}
	return err
}

func (e *events) Finish(result transfer.Result, err error) (int, error) {
	if e.err != nil {
		return 1, e.err
	}
	if e.finished {
		return 1, errors.New("JSON result already emitted")
	}
	e.finished = true

	if err == nil && !result.Published {
		err = errors.New("download ended without publishing verified output")
	}

	status, code := "completed", 0
	var message *string
	if err != nil {
		status, code = "failed", 1
		if errors.Is(err, context.Canceled) {
			status, code = "interrupted", 130
		}
		text := err.Error()
		message = &text
	}

	return code, e.write(context.Background(), resultEvent{
		Version: eventVersion, Event: "result", Elapsed: time.Since(e.start).Milliseconds(), Status: status, ExitCode: code,
		Path: optionalText(result.Path), Partial: optionalText(result.Partial), Bytes: result.Bytes,
		Total: knownTotal(e.latest.Total), Resumed: result.Resumed, Published: result.Published, Error: message,
	})
}

func (e *events) write(ctx context.Context, event any) error {
	data, err := json.Marshal(event)
	if err == nil {
		var n int
		n, err = e.out.Write(ctx, append(data, '\n'))
		if err != nil && n == 0 && ctx.Err() != nil {
			return nil
		}
	}
	if err != nil {
		e.err = fmt.Errorf("write JSON output: %w", err)
	}
	return e.err
}

func (e *events) Close() error {
	return e.out.Close()
}

func knownTotal(total int64) *int64 {
	if total < 0 {
		return nil
	}
	return &total
}

func optionalText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

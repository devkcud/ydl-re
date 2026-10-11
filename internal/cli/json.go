package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"yoinkdl/internal/transfer"
)

func downloadJSON(parent context.Context, req transfer.Request, quiet bool, out, errOut io.Writer) int {
	e, err := newEvents(out, quiet)
	if err != nil {
		return jsonFailure(parent, errOut, transfer.Result{}, err)
	}

	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	result, err := transfer.Download(ctx, req, func(s transfer.Snapshot) {
		if err := e.Update(ctx, s); err != nil {
			cancel(err)
		}
	})

	code, writeErr := e.Finish(result, err)
	if writeErr != nil && !errors.Is(err, writeErr) {
		err = errors.Join(writeErr, err)
	}
	closeErr := e.Close()
	if writeErr != nil || closeErr != nil {
		return jsonFailure(parent, errOut, result, errors.Join(err, closeErr))
	}
	return code
}

func jsonFailure(ctx context.Context, out io.Writer, result transfer.Result, err error) int {
	fmt.Fprintf(out, "JSON output failed: %v\n", err)
	if result.Published {
		fmt.Fprintf(out, "Verified output published: %q\n", result.Path)
	}
	if result.Partial != "" {
		fmt.Fprintf(out, "Partial file preserved: %q\n", result.Partial)
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return 130
	}
	return 1
}

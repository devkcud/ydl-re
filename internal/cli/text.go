package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"yoinkdl/internal/transfer"
)

func writeText(ctx context.Context, out io.Writer, text string) error {
	w, err := newRecordWriter(out)
	if err != nil {
		return err
	}

	_, err = w.Write(ctx, []byte(text))
	return errors.Join(err, w.Close())
}

func reply(ctx context.Context, out, errOut io.Writer, text string) int {
	if err := writeText(ctx, out, text); err != nil {
		label := "output failed"
		if errors.Is(ctx.Err(), context.Canceled) {
			label = "output interrupted"
		}
		writeText(context.Background(), errOut, label+": "+safeText(err.Error())+"\n")
		return outputCode(ctx, 1)
	}
	return 0
}

func diagnostic(ctx context.Context, out io.Writer, text string, code int) int {
	writeText(ctx, out, text+"\n")
	return outputCode(ctx, code)
}

func outputCode(ctx context.Context, code int) int {
	if errors.Is(ctx.Err(), context.Canceled) {
		return 130
	}
	return code
}

func downloadText(parent context.Context, req transfer.Request, quiet bool, out, errOut io.Writer) int {
	w, err := newRecordWriter(out)
	if err == nil {
		err = w.Close()
	}
	if err != nil {
		return textFailure(parent, errOut, transfer.Result{}, fmt.Errorf("prepare stdout: %w", err))
	}

	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	var update func(transfer.Snapshot)
	var display *progress
	if !quiet {
		display, err = newProgress(errOut)
		if err != nil {
			return textFailure(parent, errOut, transfer.Result{}, fmt.Errorf("prepare progress output: %w", err))
		}
		update = func(s transfer.Snapshot) {
			if err := display.Update(ctx, s); err != nil {
				cancel(err)
			}
		}
	}

	result, err := transfer.Download(ctx, req, update)
	if display != nil {
		if finishErr := display.Finish(); !errors.Is(err, finishErr) {
			err = errors.Join(err, finishErr)
		}
	}
	if err != nil {
		return textFailure(parent, errOut, result, err)
	}

	if err := writeText(context.Background(), out, safeText(result.Path)+"\n"); err != nil {
		return textFailure(parent, errOut, result, fmt.Errorf("write final path to stdout: %w", err))
	}
	return 0
}

func textFailure(ctx context.Context, out io.Writer, result transfer.Result, err error) int {
	code := outputCode(ctx, 1)
	label := "download failed"
	if code == 130 || errors.Is(err, context.Canceled) {
		code = 130
		label = "download interrupted"
	}
	writeText(context.Background(), out, label+": "+safeText(err.Error())+"\n"+stateText(result))
	return code
}

func stateText(result transfer.Result) string {
	var text strings.Builder
	if result.Published {
		fmt.Fprintf(&text, "Verified output published: %s\n", safeText(result.Path))
	}
	if result.Partial != "" {
		fmt.Fprintf(&text, "Partial file preserved: %s\n", safeText(result.Partial))
	}
	return text.String()
}

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"yoinkdl/internal/daemon"
	"yoinkdl/internal/queue"
	"yoinkdl/internal/transfer"
)

const daemonUsage = `Usage: yoinkdl daemon <start|stop|set> [options]

  start [--parallel N] [--limit R]  Run the Linux daemon in the foreground
  stop [--json]                     Request durable, draining shutdown
  set --parallel N [--json]         Change whole-job concurrency, 1..8

Start is explicit; other commands never launch a daemon. The parallel setting
persists, while the shared rate cap is fixed until restart. State/runtime paths
follow XDG settings; startup reports a persistent runtime fallback when used.
`

func daemonCommand(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		return reply(ctx, out, errOut, daemonUsage)
	}
	if len(args) == 0 || args[0] != "start" && args[0] != "stop" && args[0] != "set" {
		return diagnostic(ctx, errOut, "usage: yoinkdl daemon <start|stop|set> [options]", 2)
	}
	name := "daemon " + args[0]
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var parallel int
	var limit int64
	var json bool
	if args[0] != "start" {
		fs.BoolVar(&json, "json", false, "JSON result")
	}
	if args[0] != "stop" {
		fs.Func("parallel", "whole-job concurrency", func(value string) error {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > queue.MaxParallel || strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
				return errors.New("parallel must be a decimal integer from 1 through 8")
			}
			parallel = n
			return nil
		})
	}
	if args[0] == "start" {
		fs.Func("limit", "shared body rate cap", func(value string) error {
			var err error
			limit, err = transfer.ParseLimit(value)
			return err
		})
	}
	values, err := parseFlags(fs, args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return reply(ctx, out, errOut, daemonUsage)
	}
	if err != nil {
		return diagnostic(ctx, errOut, err.Error(), 2)
	}
	if len(values) != 0 || args[0] == "set" && parallel == 0 {
		return diagnostic(ctx, errOut, "invalid arguments; run yoinkdl "+name+" --help", 2)
	}
	if args[0] == "start" {
		return daemonStart(ctx, parallel, limit, out, errOut)
	}
	cmd := queueCall{name: name, op: "stop", args: map[string]any{}, json: json}
	if args[0] == "set" {
		cmd.op = "set_parallel"
		cmd.args = map[string]any{"parallel": parallel}
	}
	return runQueue(ctx, cmd, out, errOut)
}

func daemonStart(ctx context.Context, parallel int, limit int64, out, errOut io.Writer) int {
	if err := ctx.Err(); err != nil {
		failure := queueFailure(err)
		return diagnostic(context.Background(), errOut, "daemon start failed: "+failure.Message, queueExit(ctx, failure))
	}
	w, err := newRecordWriter(out)
	if err != nil {
		return diagnostic(ctx, errOut, "output unavailable; daemon was not started", 1)
	}
	defer w.Close()
	paths, err := daemon.Resolve()
	var server *daemon.Server
	if err == nil {
		server, err = daemon.New(daemon.Options{Paths: paths, Parallel: parallel, Limit: limit})
	}
	if err != nil {
		failure := queueFailure(err)
		return diagnostic(context.Background(), errOut, "daemon start failed: "+failure.Message, queueExit(ctx, failure))
	}
	text := fmt.Sprintf("Daemon listening: %s\nState directory: %s\n", safeText(paths.Socket()), safeText(paths.State))
	if paths.Fallback {
		text += "Runtime directory: " + safeText(paths.Runtime) + " (persistent fallback)\n"
	}
	_, err = w.Write(ctx, []byte(text))
	err = errors.Join(err, w.Close())
	if err != nil {
		closeErr := server.Close()
		text := "startup output failed; daemon stopped before admitting jobs"
		if closeErr != nil {
			text += "; " + queueFailure(closeErr).Message
		}
		return diagnostic(context.Background(), errOut, text, outputCode(ctx, 1))
	}
	if err := server.Run(ctx); err != nil {
		failure := queueFailure(err)
		return diagnostic(context.Background(), errOut, "daemon stopped: "+failure.Message, queueExit(ctx, failure))
	}
	return 0
}

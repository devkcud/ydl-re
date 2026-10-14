package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"yoinkdl/internal/daemon"
	"yoinkdl/internal/queue"
	"yoinkdl/internal/transfer"
)

type queueCall struct {
	name string
	op   string
	id   string
	args any
	json bool
}

func add(ctx context.Context, args []string, out, errOut io.Writer) int {
	req, opts, err := parseTransfer(args, "add")
	if errors.Is(err, flag.ErrHelp) {
		return reply(ctx, out, errOut, queueUsage("add"))
	}
	if err != nil {
		return diagnostic(ctx, errOut, err.Error(), 2)
	}
	req, err = transfer.Normalize(req)
	if err != nil {
		return diagnostic(ctx, errOut, err.Error(), 2)
	}
	req.Output, err = filepath.Abs(req.Output)
	if err != nil {
		return diagnostic(ctx, errOut, "cannot resolve output from the current directory", 2)
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return diagnostic(ctx, errOut, "cannot generate a job ID", 1)
	}
	id := hex.EncodeToString(token[:])
	j, err := queue.NewJob(id, 1, req)
	if err != nil {
		return diagnostic(ctx, errOut, err.Error(), 2)
	}
	return runQueue(ctx, queueCall{
		name: "add", op: "add", id: id, json: opts.json,
		args: map[string]any{"id": id, "request": j.Request},
	}, out, errOut)
}

func queueCommand(ctx context.Context, name string, args []string, out, errOut io.Writer) int {
	cmd := queueCall{name: name, op: name, args: map[string]any{}}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&cmd.json, "json", false, "JSON result")
	values, err := parseFlags(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return reply(ctx, out, errOut, queueUsage(name))
	}
	if err != nil {
		return diagnostic(ctx, errOut, err.Error(), 2)
	}
	if name == "list" {
		if len(values) != 0 {
			return diagnostic(ctx, errOut, "list does not accept positional arguments", 2)
		}
	} else {
		if len(values) != 1 || !jobID(values[0]) {
			return diagnostic(ctx, errOut, name+" requires one job ID of 32 lowercase hex characters", 2)
		}
		cmd.id = values[0]
		cmd.args = map[string]any{"id": cmd.id}
	}
	return runQueue(ctx, cmd, out, errOut)
}

func runQueue(ctx context.Context, cmd queueCall, out, errOut io.Writer) int {
	w, err := newRecordWriter(out)
	if err != nil {
		return diagnostic(ctx, errOut, "output unavailable; request was not sent", 1)
	}
	defer w.Close()
	paths, err := daemon.Resolve()
	var result daemon.Reply
	if err == nil {
		result, err = daemon.Call(ctx, paths, cmd.op, cmd.args)
	}
	if err == nil && cmd.id != "" {
		var job struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(result.Result, &job) != nil || job.ID != cmd.id {
			err = errors.Join(queue.ErrOutcomeUnknown, daemon.ErrProtocol)
			result.Result = nil
		}
	}
	code := 0
	var failure *daemon.Failure
	if err != nil {
		failure = queueFailure(err)
		code = queueExit(ctx, failure)
		if cmd.op == "add" && errors.Is(err, daemon.ErrProtocol) && !errors.Is(err, queue.ErrOutcomeUnknown) {
			code = 2
		}
	}
	var data []byte
	if cmd.json {
		data, err = json.Marshal(queueRecord{
			Version: 1, Command: cmd.name, ID: optionalText(cmd.id), ExitCode: code, Result: result.Result, Error: failure,
		})
		data = append(data, '\n')
	} else if failure != nil {
		return diagnostic(context.Background(), errOut, queueErrorText(cmd, failure), code)
	} else {
		var text string
		text, err = queueText(cmd, result.Result)
		data = []byte(text)
	}
	writeCtx := ctx
	if failure != nil {
		writeCtx = context.Background()
	}
	if err == nil {
		_, err = w.Write(writeCtx, data)
	}
	err = errors.Join(err, w.Close())
	if err != nil {
		text := "output failed"
		if errors.Is(ctx.Err(), context.Canceled) {
			text = "output interrupted"
		}
		if cmd.op != "list" && failure == nil {
			text += "; operation was accepted; run yoinkdl list"
		} else if failure != nil {
			text += "; " + failure.Message
		}
		if cmd.id != "" {
			text += "\nJob ID: " + cmd.id
		}
		writeText(context.Background(), errOut, text+"\n")
		return outputCode(ctx, max(code, 1))
	}
	return code
}

func jobID(id string) bool {
	if len(id) != 32 || strings.ToLower(id) != id {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func queueUsage(name string) string {
	if name == "add" {
		return `Usage: yoinkdl add <url> [options]

  -o, --output <path>      Output relative to this client's directory
  -c, --connections <n>    Per-job workers, 1..32 (default: 4)
      --retries <n>        Additional attempts, 0..10 (default: 3)
      --timeout <duration> Network inactivity timeout (default: 30s)
      --limit <rate>       Per-job body cap, e.g. 5MB/s
      --resume             Require an existing compatible checkpoint
      --json               Print one JSON result
  -h, --help               Show this help

An explicitly started daemon is required. Success prints the new job ID;
it acknowledges queue admission, not completion. Files are never overwritten.
Source URLs are stored privately and omitted from all status output.
`
	}
	if name == "list" {
		return "Usage: yoinkdl list [--json]\n\nPoll daemon policy and retained jobs without displaying source URLs.\n"
	}
	return fmt.Sprintf("Usage: yoinkdl %s <id> [--json]\n\nUse the 32-character job ID from add or list. Files are preserved.\n", name)
}

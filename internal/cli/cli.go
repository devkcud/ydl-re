package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"yoinkdl/internal/storage"
	"yoinkdl/internal/transfer"
)

const usage = `Usage: yoinkdl <command>

Commands:
  download <url> [options]  Download an HTTP(S) file
  inspect <output>          Show saved transfer metadata
  version                   Show the installed version
  help                      Show this help

Run yoinkdl download --help for download options.
`

const downloadUsage = `Usage: yoinkdl download <url> [options]

Options may appear before or after the URL.

  -o, --output <path>      Final file path (default: safe URL filename)
  -c, --connections <n>    Maximum workers, 1..32 (default: 4)
      --retries <n>        Additional attempts per request/range, 0..10 (default: 3)
      --limit <rate>       Aggregate body rate, e.g. 5MB/s (default: unlimited)
      --overwrite          Replace an existing completed file
      --resume             Require a saved compatible transfer
      --json               Emit JSON progress and a terminal result to stdout
      --quiet              Suppress progress; keep errors and final output
      --timeout <duration> Network inactivity timeout (default: 30s)
  -h, --help               Show this help

The output directory must already exist. Compatible saved transfers resume
automatically; --resume requires saved state. Incompatible partials are preserved.
One connection uses a direct stream for new files and one worker for saved ranges.
Interrupted sequential bodies keep their checkpoints for a later resume.
Rates use positive integers and B/s, KB/s, MB/s, GB/s, KiB/s, MiB/s, or GiB/s.
JSON uses one object per line; --quiet keeps only the terminal result.
`

const inspectUsage = `Usage: yoinkdl inspect <output>

Read local checkpoint metadata without contacting the server or changing files.
This does not verify stored bytes or current remote identity.
`

func Run(ctx context.Context, args []string, out, errOut io.Writer, version string) int {
	if len(args) == 0 {
		fmt.Fprint(out, usage)
		return 0
	}

	switch args[0] {
	case "help", "--help", "-h":
		if len(args) == 2 {
			switch args[1] {
			case "download":
				fmt.Fprint(out, downloadUsage)
				return 0
			case "inspect":
				fmt.Fprint(out, inspectUsage)
				return 0
			}
		}

		if len(args) != 1 {
			fmt.Fprintln(errOut, "usage: yoinkdl help [download|inspect]")
			return 2
		}

		fmt.Fprint(out, usage)
		return 0

	case "version":
		if len(args) != 1 {
			fmt.Fprintln(errOut, "usage: yoinkdl version")
			return 2
		}

		if _, err := fmt.Fprintln(out, "yoinkdl "+safeText(version)); err != nil {
			return 1
		}

		return 0

	case "download":
		return download(ctx, args[1:], out, errOut)

	case "inspect":
		return inspect(args[1:], out, errOut)

	default:
		fmt.Fprintln(errOut, "unknown command; run yoinkdl help")
		return 2
	}
}

func download(ctx context.Context, args []string, out, errOut io.Writer) int {
	req, opts, err := parseDownload(args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(out, downloadUsage)
		return 0
	}

	if err != nil {
		fmt.Fprintln(errOut, safeText(err.Error()))
		return 2
	}
	if opts.json {
		return downloadJSON(ctx, req, opts.quiet, out, errOut)
	}

	var update func(transfer.Snapshot)
	var display *progress

	if !opts.quiet {
		display = newProgress(errOut)
		update = display.Update
	}

	result, err := transfer.Download(ctx, req, update)
	if display != nil {
		display.Finish()
	}

	if err != nil {
		code := 1
		label := "download failed"

		if errors.Is(err, context.Canceled) {
			code = 130
			label = "download interrupted"
		}

		fmt.Fprintf(errOut, "%s: %s\n", label, safeText(err.Error()))

		if result.Published {
			fmt.Fprintf(errOut, "Verified output published: %s\n", safeText(result.Path))
		}

		if result.Partial != "" {
			fmt.Fprintf(errOut, "Partial file preserved: %s\n", safeText(result.Partial))
		}

		return code
	}

	if _, err := fmt.Fprintln(out, safeText(result.Path)); err != nil {
		fmt.Fprintln(errOut, "download completed, but the final path could not be written to stdout")
		return 1
	}

	return 0
}

type outputOptions struct {
	quiet bool
	json  bool
}

func parseDownload(args []string) (transfer.Request, outputOptions, error) {
	var req transfer.Request
	var opts outputOptions
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&req.Output, "output", "", "output path")
	fs.StringVar(&req.Output, "o", "", "output path")
	fs.BoolVar(&req.Overwrite, "overwrite", false, "replace completed output")
	fs.BoolVar(&req.Resume, "resume", false, "require existing transfer state")
	fs.BoolVar(&opts.quiet, "quiet", false, "suppress progress")
	fs.BoolVar(&opts.json, "json", false, "JSON progress and result")
	fs.DurationVar(&req.Timeout, "timeout", transfer.DefaultTimeout, "network inactivity timeout")

	fs.Func("limit", "aggregate body rate", func(value string) error {
		n, err := transfer.ParseLimit(value)
		if err != nil {
			return err
		}

		req.Limit = n
		return nil
	})

	req.Connections = transfer.DefaultConnections
	connections := func(value string) error {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > transfer.MaxConnections ||
			strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return fmt.Errorf("connections must be a decimal integer between 1 and %d", transfer.MaxConnections)
		}

		req.Connections = n
		return nil
	}
	fs.Func("connections", "maximum workers", connections)
	fs.Func("c", "maximum workers", connections)
	retries := transfer.DefaultRetries
	req.Retries = &retries
	fs.Func("retries", "additional attempts", func(value string) error {
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 || n > transfer.MaxRetries ||
			strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return fmt.Errorf("retries must be a decimal integer between 0 and %d", transfer.MaxRetries)
		}

		*req.Retries = n
		return nil
	})

	var options, urls []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			urls = append(urls, args[i+1:]...)
			break
		}

		if arg == "-h" || arg == "--help" {
			return req, opts, flag.ErrHelp
		}

		if !strings.HasPrefix(arg, "-") || arg == "-" {
			urls = append(urls, arg)
			continue
		}

		name, _, inline := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		option := fs.Lookup(name)

		if option == nil {
			return req, opts, errors.New("unknown option; run yoinkdl download --help")
		}

		options = append(options, arg)
		boolean, ok := option.Value.(interface{ IsBoolFlag() bool })

		if !inline && (!ok || !boolean.IsBoolFlag()) {
			i++

			if i >= len(args) {
				return req, opts, fmt.Errorf("option --%s requires a value", name)
			}

			options = append(options, args[i])
		}
	}

	if err := fs.Parse(options); err != nil {
		return req, opts, err
	}

	if len(urls) != 1 {
		return req, opts, errors.New("download requires exactly one HTTP(S) URL")
	}

	if req.Timeout <= 0 {
		return req, opts, errors.New("timeout must be positive")
	}

	var emptyOutput bool
	fs.Visit(func(option *flag.Flag) {
		if (option.Name == "output" || option.Name == "o") && req.Output == "" {
			emptyOutput = true
		}
	})

	if emptyOutput {
		return req, opts, errors.New("output path must not be empty")
	}

	req.URL = urls[0]
	return req, opts, nil
}

func inspect(args []string, out, errOut io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprint(out, inspectUsage)
		return 0
	}

	if len(args) != 1 {
		fmt.Fprintln(errOut, "inspect requires exactly one output path")
		return 2
	}

	m, err := storage.Inspect(args[0])
	if err != nil {
		fmt.Fprintf(errOut, "inspect failed: %s\n", safeText(err.Error()))
		return 1
	}

	total := "unknown"
	if m.Length >= 0 {
		total = fmt.Sprintf("%d", m.Length)
	}

	resume := "unavailable for this representation"
	if m.State == storage.StateReady {
		resume = "local publication pending validation"
	} else if m.Resumable() {
		resume = "eligible after local and server validation"
	}

	_, err = fmt.Fprintf(out,
		"Destination: %q\nState: %s\nSaved bytes: %d\nTotal bytes: %s\nEncoding: %s\nResume: %s\n",
		m.Destination, m.State, m.Completed, total, safeText(m.Encoding), resume)

	if err != nil {
		return 1
	}

	return 0
}

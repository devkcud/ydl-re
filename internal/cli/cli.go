package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
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
      --overwrite          Replace an existing completed file
      --resume             Require a saved compatible transfer
      --quiet              Suppress progress; keep errors and final path
      --timeout <duration> Network inactivity timeout (default: 30s)
  -h, --help               Show this help

The output directory must already exist. Compatible saved transfers resume
automatically; --resume requires saved state. Incompatible partials are preserved.
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

		if _, err := fmt.Fprintln(out, "yoinkdl "+version); err != nil {
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
	req, quiet, err := parseDownload(args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(out, downloadUsage)
		return 0
	}

	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}

	var update func(transfer.Snapshot)
	var display *progress

	if !quiet {
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

		fmt.Fprintf(errOut, "%s: %v\n", label, err)

		if result.Published {
			fmt.Fprintf(errOut, "Verified output published: %s\n", result.Path)
		}

		if result.Partial != "" {
			fmt.Fprintf(errOut, "Partial file preserved: %s\n", result.Partial)
		}

		return code
	}

	if _, err := fmt.Fprintln(out, result.Path); err != nil {
		fmt.Fprintln(errOut, "download completed, but the final path could not be written to stdout")
		return 1
	}

	return 0
}

func parseDownload(args []string) (transfer.Request, bool, error) {
	var req transfer.Request
	var quiet bool
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&req.Output, "output", "", "output path")
	fs.StringVar(&req.Output, "o", "", "output path")
	fs.BoolVar(&req.Overwrite, "overwrite", false, "replace completed output")
	fs.BoolVar(&req.Resume, "resume", false, "require existing transfer state")
	fs.BoolVar(&quiet, "quiet", false, "suppress progress")
	fs.DurationVar(&req.Timeout, "timeout", transfer.DefaultTimeout, "network inactivity timeout")

	var options, urls []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			urls = append(urls, args[i+1:]...)
			break
		}

		if arg == "-h" || arg == "--help" {
			return req, quiet, flag.ErrHelp
		}

		if !strings.HasPrefix(arg, "-") || arg == "-" {
			urls = append(urls, arg)
			continue
		}

		name, _, inline := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		option := fs.Lookup(name)

		if option == nil {
			return req, quiet, errors.New("unknown option; run yoinkdl download --help")
		}

		options = append(options, arg)
		boolean, ok := option.Value.(interface{ IsBoolFlag() bool })

		if !inline && (!ok || !boolean.IsBoolFlag()) {
			i++

			if i >= len(args) {
				return req, quiet, fmt.Errorf("option --%s requires a value", name)
			}

			options = append(options, args[i])
		}
	}

	if err := fs.Parse(options); err != nil {
		return req, quiet, err
	}

	if len(urls) != 1 {
		return req, quiet, errors.New("download requires exactly one HTTP(S) URL")
	}

	if req.Timeout <= 0 {
		return req, quiet, errors.New("timeout must be positive")
	}

	var emptyOutput bool
	fs.Visit(func(option *flag.Flag) {
		if (option.Name == "output" || option.Name == "o") && req.Output == "" {
			emptyOutput = true
		}
	})

	if emptyOutput {
		return req, quiet, errors.New("output path must not be empty")
	}

	req.URL = urls[0]
	return req, quiet, nil
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
		fmt.Fprintf(errOut, "inspect failed: %v\n", err)
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
		m.Destination, m.State, m.Completed, total, m.Encoding, resume)

	if err != nil {
		return 1
	}

	return 0
}

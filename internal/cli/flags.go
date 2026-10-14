package cli

import (
	"flag"
	"fmt"
	"strings"
)

func parseFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var options []struct{ name, value string }
	var values []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			values = append(values, args[i+1:]...)
			break
		}

		if arg == "-h" || arg == "--help" {
			return nil, flag.ErrHelp
		}

		if !strings.HasPrefix(arg, "-") || arg == "-" {
			values = append(values, arg)
			continue
		}

		name, value, inline := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-"), "=")
		option := fs.Lookup(name)

		if option == nil {
			return nil, fmt.Errorf("unknown option; run yoinkdl %s --help", fs.Name())
		}

		boolean, ok := option.Value.(interface{ IsBoolFlag() bool })

		if !inline {
			value = "true"
			if !ok || !boolean.IsBoolFlag() {
				i++

				if i >= len(args) {
					return nil, fmt.Errorf("option --%s requires a value", name)
				}

				value = args[i]
			}
		}

		options = append(options, struct{ name, value string }{name, value})
	}

	for _, option := range options {
		if err := fs.Set(option.name, option.value); err != nil {
			return nil, fmt.Errorf("invalid value for --%s; run yoinkdl %s --help", option.name, fs.Name())
		}
	}

	return values, nil
}

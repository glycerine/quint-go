package cli

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"

	quint "github.com/glycerine/quint-go/pkg"
)

func replParseOptions(args []string) (REPLLoopOptions, string, error) {
	options := REPLLoopOptions{Verbosity: 2}
	args, err := cliNormalizeOptions("repl", args)
	if err != nil {
		return options, "", err
	}
	flags := flag.NewFlagSet("repl", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&options.Quiet, "quiet", false, "suppress banners and prompts")
	flags.BoolVar(&options.Quiet, "q", false, "suppress banners and prompts")
	flags.IntVar(&options.Verbosity, "verbosity", 2, "output level from 0 to 5")
	flags.StringVar(&options.Require, "require", "", "preload filename[::module]")
	flags.StringVar(&options.Require, "r", "", "preload filename[::module]")
	seed := flags.String("seed", "", "random seed")
	backend := flags.String("backend", "go", "evaluation backend (go)")
	flags.Usage = func() {}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			var output bytes.Buffer
			fmt.Fprintln(&output, "Usage: quint repl [options] [commands...]")
			flags.SetOutput(&output)
			flags.PrintDefaults()
			return options, output.String(), nil
		}
		return options, "", err
	}
	if *backend != "go" {
		return options, "", fmt.Errorf("repl: unsupported backend %q; use go", *backend)
	}
	if options.Verbosity < 0 || options.Verbosity > 5 {
		return options, "", fmt.Errorf("repl: --verbosity must be between 0 and 5")
	}
	if *seed != "" {
		value, err := quint.ParseSeed(*seed)
		if err != nil {
			return options, "", err
		}
		options.Seed = &value
	}
	options.Commands = flags.Args()
	return options, "", nil
}

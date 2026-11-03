package cli

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

type cliFlagSpec struct {
	kind         byte // string, number, boolean, or array
	defaultValue string
	hasDefault   bool
}

type cliFlagValue struct {
	name, first, value string
	values             []string
	kind               byte
	omit               bool
}

// cliNormalizeOptions preserves yargs' common token rules before the command's
// own validation: aliases, interspersed options, boolean negation, greedy array
// values, missing scalar defaults, and duplicate scalar rejection. Execution
// limits and backend choices remain with the command that owns them.
func cliNormalizeOptions(command string, args []string) ([]string, error) {
	help, version, remaining := coreInformationalOptions(args)
	if help {
		return []string{"--help"}, nil
	}
	if version {
		return []string{"--version"}, nil
	}
	args = remaining
	specs := cliFlagSpecs(command)
	flags := insdict.NewDict[string, *cliFlagValue]()
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		argument := args[i]
		if argument == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if coreOptionValueArgument(argument) {
			positionals = append(positionals, argument)
			continue
		}
		raw, value, inline := strings.Cut(strings.TrimPrefix(argument, "--"), "=")
		if !strings.HasPrefix(argument, "--") {
			raw = strings.TrimPrefix(raw, "-")
			if command == "repl" && (raw == "q" || raw == "r") {
				// Short aliases are declared only by the REPL builder.
			} else {
				return nil, coreUnknownShortArguments(argument)
			}
		}
		name, negated := raw, false
		if strings.HasPrefix(name, "no-") && !inline {
			name, negated = strings.TrimPrefix(name, "no-"), true
		}
		name = cliDashedFlag(name)
		if command == "repl" {
			if name == "q" {
				name = "quiet"
			} else if name == "r" {
				name = "require"
			}
		}
		spec, known := specs[name]
		if !known || negated && spec.kind != 'b' {
			unknown := raw
			if strings.Contains(raw, "-") {
				return nil, fmt.Errorf("Unknown arguments: %s, %s", unknown, cliCamelFlag(unknown))
			}
			return nil, fmt.Errorf("Unknown argument: %s", unknown)
		}
		previous := flags.Get(name)
		if previous != nil && spec.kind != 'b' && spec.kind != 'a' {
			return nil, fmt.Errorf("--%s can not be specified more than once", previous.first)
		}
		flag := &cliFlagValue{name: name, first: raw, kind: spec.kind}
		if previous != nil {
			flag.first = previous.first
			flag.values = previous.values
		}
		switch spec.kind {
		case 'b':
			if negated {
				value = "false"
			} else if !inline {
				value = "true"
				if i+1 < len(args) && (args[i+1] == "true" || args[i+1] == "false") {
					i++
					value = args[i]
				}
			}
			flag.value = strconv.FormatBool(value == "true")
		case 'a':
			if inline {
				flag.values = append(flag.values, value)
			}
			for i+1 < len(args) && args[i+1] != "--" && coreOptionValueArgument(args[i+1]) {
				i++
				flag.values = append(flag.values, args[i])
			}
		default:
			if !inline {
				if i+1 < len(args) && args[i+1] != "--" && coreOptionValueArgument(args[i+1]) {
					i++
					value = args[i]
				} else if spec.hasDefault {
					value = spec.defaultValue
				} else if spec.kind == 'n' {
					flag.omit = true
				}
			}
			if name == "seed" {
				seed, err := quint.ParseSeed(value)
				if err != nil {
					return nil, err
				}
				value = seed.String()
			}
			if spec.kind == 'n' && !flag.omit {
				number := coreNumberOption(value)
				if !math.IsNaN(number) && !math.IsInf(number, 0) && math.Trunc(number) == number {
					value = strconv.FormatFloat(number, 'f', -1, 64)
				}
			}
			flag.value = value
		}
		flags.Set(name, flag)
	}
	// The hidden test --output option is an override in the upstream handler,
	// rather than a parser alias. Its value wins in either argument order.
	if command == "test" && flags.Get("output") != nil {
		flags.Del("out-itf")
		flags.Get("output").name = "out-itf"
	}
	output := make([]string, 0, len(args)+1)
	for _, flag := range flags.All() {
		if flag.omit {
			continue
		}
		if flag.kind == 'a' {
			for _, value := range flag.values {
				output = append(output, "--"+flag.name+"="+value)
			}
		} else {
			output = append(output, "--"+flag.name+"="+flag.value)
		}
	}
	if len(positionals) > 0 {
		output = append(output, "--")
		output = append(output, positionals...)
	}
	return output, nil
}

func cliFlagSpecs(command string) map[string]cliFlagSpec {
	flags := map[string]cliFlagSpec{}
	add := func(kind byte, names ...string) {
		for _, name := range names {
			flags[name] = cliFlagSpec{kind: kind}
		}
	}
	defaults := func(kind byte, values ...string) {
		for i := 0; i < len(values); i += 2 {
			flags[values[i]] = cliFlagSpec{kind: kind, defaultValue: values[i+1], hasDefault: true}
		}
	}
	if command != "repl" {
		add('s', "out")
	}
	switch command {
	case "parse":
		add('s', "source-map")
	case "compile", "verify":
		add('s', "main", "init", "step", "invariant", "temporal", "apalache-version", "server-endpoint")
		if command == "compile" {
			defaults('s', "target", "json")
			add('b', "flatten")
			defaults('n', "verbosity", "2")
		} else {
			defaults('s', "init", "init", "step", "step", "backend", "apalache")
			add('s', "inductive-invariant", "out-itf", "apalache-config", "tlc-config")
			add('a', "invariants")
			defaults('n', "max-steps", "10")
			add('n', "verbosity")
			add('b', "random-transitions")
		}
	case "run", "test":
		add('s', "main", "out-itf", "seed")
		defaults('s', "backend", "go")
		add('n', "max-samples")
		defaults('n', "verbosity", "2")
		if command == "run" {
			defaults('s', "init", "init", "step", "step", "invariant", "true")
			defaults('n', "max-steps", "20", "n-traces", "1")
			add('n', "n-threads")
			add('a', "invariants", "witnesses", "hide")
			add('b', "mbt")
		} else {
			add('s', "match", "output")
		}
	case "repl":
		add('s', "require", "seed")
		defaults('s', "backend", "go")
		defaults('n', "verbosity", "2")
		add('b', "quiet")
	}
	return flags
}

func cliDashedFlag(name string) string {
	var out strings.Builder
	for _, char := range name {
		if char >= 'A' && char <= 'Z' {
			out.WriteByte('-')
			out.WriteRune(unicode.ToLower(char))
		} else {
			out.WriteRune(char)
		}
	}
	return out.String()
}

func cliCamelFlag(name string) string {
	var out strings.Builder
	upper := false
	for _, char := range name {
		if char == '-' {
			upper = true
		} else if upper {
			out.WriteRune(unicode.ToUpper(char))
			upper = false
		} else {
			out.WriteRune(char)
		}
	}
	return out.String()
}

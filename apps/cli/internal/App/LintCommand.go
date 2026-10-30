package app

import (
	"errors"
	"strings"

	lint "github.com/Khaymah/dhow/apps/cli/internal/Lint"
	output "github.com/Khaymah/dhow/apps/cli/internal/Output"
)

const (
	FormatText = "text"
	FormatJSON = "json"

	optionFormat = "format"
	optionLang   = "lang"
)

type lintOptions struct {
	path   string
	format string
	lang   string
	help   bool
}

type usageError struct {
	key output.Key
	arg string
}

func runLint(args []string, env Env) int {
	opts, bad := parseLint(args)
	c := catalog(opts.lang, env)
	if opts.help {
		return usage(env.Out, c, ExitValid)
	}
	if bad != nil {
		msg := c.T(bad.key)
		if bad.arg != "" {
			msg = c.T(bad.key, output.Printable(bad.arg))
		}
		say(env.Err, msg, c.T(output.KeyHint))
		return ExitUsage
	}
	path := lint.Resolve(opts.path)
	res, err := lint.File(path)
	if err != nil {
		say(env.Err, c.T(fileErrorKey(err), output.Printable(path)))
		return ExitUsage
	}
	render := output.Text
	if opts.format == FormatJSON {
		render = output.JSON
	}
	if err := render(env.Out, c, res); err != nil {
		return ExitUsage
	}
	if res.Errors() > 0 {
		return ExitInvalid
	}
	return ExitValid
}

func fileErrorKey(err error) output.Key {
	switch {
	case errors.Is(err, lint.ErrNotFound):
		return output.KeyFileNotFound
	case errors.Is(err, lint.ErrNotRegular):
		return output.KeyFileNotRegular
	case errors.Is(err, lint.ErrSymlink):
		return output.KeyFileSymlink
	case errors.Is(err, lint.ErrNoAccess):
		return output.KeyFileNoAccess
	default:
		return output.KeyFileUnreadable
	}
}

func parseLint(args []string) (lintOptions, *usageError) {
	opts := lintOptions{format: FormatText}
	var bad *usageError
	note := func(e *usageError) {
		if bad == nil {
			bad = e
		}
	}
	var paths []string
	onlyPaths := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if onlyPaths || arg == "-" || !strings.HasPrefix(arg, "-") {
			paths = append(paths, arg)
			continue
		}
		if arg == "--" {
			onlyPaths = true
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-"), "=")
		switch {
		case (name == "h" || name == "help") && !hasValue:
			opts.help = true
			continue
		case name != optionFormat && name != optionLang:
			note(&usageError{key: output.KeyUnknownOption, arg: arg})
			continue
		case !hasValue && i+1 >= len(args):
			note(&usageError{key: output.KeyOptionValue, arg: "--" + name})
			continue
		case !hasValue:
			i++
			value = args[i]
		}
		if name == optionFormat {
			opts.format = value
		} else {
			opts.lang = value
		}
	}
	if opts.lang != "" && !output.ValidLang(opts.lang) {
		note(&usageError{key: output.KeyLangInvalid})
	}
	if opts.format != FormatText && opts.format != FormatJSON {
		note(&usageError{key: output.KeyFormatInvalid})
	}
	if len(paths) > 1 {
		note(&usageError{key: output.KeyTooManyPaths})
	}
	if len(paths) > 0 {
		opts.path = paths[0]
	}
	return opts, bad
}

//go:build linux

package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	exitEscaped = 1
	exitUsage   = 2
	exitBlocked = 3
	exitOutside = 4
	sandboxMark = "/run/dhow/agent"
)

type result struct {
	name    string
	escaped bool
	detail  string
}

func blocked(name string) result { return result{name: name} }

func escaped(name, format string, args ...any) result {
	return result{name: name, escaped: true, detail: fmt.Sprintf(format, args...)}
}

func blockedUnless(name string, err error, what string) result {
	if err == nil {
		return escaped(name, "%s succeeded", what)
	}
	return blocked(name)
}

func report(results []result) int {
	code := 0
	for _, r := range results {
		if r.escaped {
			fmt.Printf("ESCAPED %s: %s\n", r.name, r.detail)
			code = exitEscaped
			continue
		}
		fmt.Printf("blocked %s\n", r.name)
	}
	return code
}

func seconds(arg string) time.Duration {
	n, err := strconv.Atoi(arg)
	if err != nil || n <= 0 {
		return time.Second
	}
	return time.Duration(n) * time.Second
}

func argOr(args []string, i int, fallback string) string {
	if len(args) > i {
		return args[i]
	}
	return fallback
}

func run(args []string) int {
	if len(args) == 0 {
		return exitUsage
	}
	rest := args[1:]
	switch args[0] {
	case "escape":
		return report(escapeProbes())
	case "capeff":
		return report([]result{capabilities()})
	case "marker":
		return marker(argOr(rest, 0, ""), seconds(argOr(rest, 1, "20")))
	case "hunt":
		return report(hunt(argOr(rest, 0, "")))
	case "forkbomb":
		return forkbomb()
	case "oom":
		return oom()
	case "cpu":
		return cpu(seconds(argOr(rest, 0, "3")))
	case "fill":
		return fill(argOr(rest, 0, "/workspace"))
	case "sleep":
		time.Sleep(seconds(argOr(rest, 0, "1")))
		return 0
	case "ignoreterm":
		return ignoreTerm(seconds(argOr(rest, 0, "600")))
	default:
		return exitUsage
	}
}

func main() {
	if _, err := os.Stat(sandboxMark); err != nil {
		fmt.Fprintln(os.Stderr, "escape-probe runs only inside a Dhow sandbox: its probes change the host when they succeed")
		os.Exit(exitOutside)
	}
	os.Exit(run(os.Args[1:]))
}

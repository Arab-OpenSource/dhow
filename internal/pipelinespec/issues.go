package pipelinespec

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

type Position struct {
	Line   int
	Column int
}

type Issue struct {
	Code     string
	Severity Severity
	Path     string
	Position Position
	Message  string
}

func (i Issue) String() string {
	var b strings.Builder
	if i.Position.Line > 0 {
		b.WriteString(strconv.Itoa(i.Position.Line))
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(i.Position.Column))
		b.WriteByte(' ')
	}
	b.WriteString(string(i.Severity))
	b.WriteByte(' ')
	b.WriteString(i.Code)
	if i.Path != "" {
		b.WriteString(" at ")
		b.WriteString(i.Path)
	}
	b.WriteString(": ")
	b.WriteString(i.Message)
	return b.String()
}

type Error struct {
	Issues []Issue
}

func (e *Error) Error() string {
	parts := make([]string, 0, len(e.Issues))
	for _, i := range e.Issues {
		parts = append(parts, i.String())
	}
	return "invalid pipeline: " + strings.Join(parts, "; ")
}

func HasErrors(issues []Issue) bool {
	return slices.ContainsFunc(issues, func(i Issue) bool { return i.Severity == SeverityError })
}

func Errors(issues []Issue) []Issue {
	out := make([]Issue, 0, len(issues))
	for _, i := range issues {
		if i.Severity == SeverityError {
			out = append(out, i)
		}
	}
	return out
}

type issueList struct {
	positions map[string]Position
	items     []Issue
}

func (l *issueList) pos(path string) Position {
	for p := path; p != ""; p = parentPath(p) {
		if pos, ok := l.positions[p]; ok {
			return pos
		}
	}
	return l.positions[""]
}

func (l *issueList) add(code, path string) {
	l.items = append(l.items, Issue{
		Code: code, Severity: SeverityError, Path: path, Position: l.pos(path), Message: message(code),
	})
}

func (l *issueList) warn(code, path string) {
	l.items = append(l.items, Issue{
		Code: code, Severity: SeverityWarning, Path: path, Position: l.pos(path), Message: message(code),
	})
}

func (l *issueList) addAt(code, path string, pos Position) {
	l.items = append(l.items, Issue{Code: code, Severity: SeverityError, Path: path, Position: pos, Message: message(code)})
}

func (l *issueList) sorted() []Issue {
	out := slices.Clone(l.items)
	slices.SortStableFunc(out, func(a, b Issue) int {
		return cmp.Or(
			cmp.Compare(a.Position.Line, b.Position.Line),
			cmp.Compare(a.Position.Column, b.Position.Column),
			cmp.Compare(a.Path, b.Path),
			cmp.Compare(a.Code, b.Code),
		)
	})
	out = slices.CompactFunc(out, func(a, b Issue) bool { return a == b })
	if len(out) > MaxIssues {
		out = out[:MaxIssues]
	}
	return out
}

func parentPath(p string) string {
	i := strings.LastIndexAny(p, ".[")
	if i <= 0 {
		return ""
	}
	return p[:i]
}

func joinPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

func indexPath(parent string, i int) string {
	return parent + "[" + strconv.Itoa(i) + "]"
}

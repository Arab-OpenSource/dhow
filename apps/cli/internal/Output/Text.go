package output

import (
	"io"
	"strconv"
	"strings"

	lint "github.com/Khaymah/dhow/apps/cli/internal/Lint"
	"github.com/Khaymah/dhow/internal/pipelinespec"
)

func Text(w io.Writer, c Catalog, r lint.Result) error {
	file := Printable(r.Path)
	var b strings.Builder
	for _, i := range r.Issues {
		writeIssue(&b, c, file, i)
	}
	switch {
	case r.Errors() > 0:
		b.WriteString(c.T(KeyInvalid, file, r.Errors(), r.Warnings()))
	case r.Warnings() > 0:
		b.WriteString(c.T(KeyValidWarnings, file, r.Warnings()))
	default:
		b.WriteString(c.T(KeyValid, file))
	}
	b.WriteByte('\n')
	_, err := io.WriteString(w, b.String())
	return err
}

func writeIssue(b *strings.Builder, c Catalog, file string, i pipelinespec.Issue) {
	b.WriteString(file)
	b.WriteByte(':')
	if i.Position.Line > 0 {
		b.WriteString(strconv.Itoa(i.Position.Line))
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(i.Position.Column))
		b.WriteByte(':')
	}
	b.WriteByte(' ')
	b.WriteString(c.Severity(i.Severity))
	b.WriteByte(' ')
	b.WriteString(i.Code)
	if i.Path != "" {
		b.WriteByte(' ')
		b.WriteString(c.T(KeyAt))
		b.WriteByte(' ')
		b.WriteString(Printable(i.Path))
	}
	b.WriteString(": ")
	b.WriteString(c.Issue(i))
	b.WriteByte('\n')
}

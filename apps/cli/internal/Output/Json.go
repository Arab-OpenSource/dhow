package output

import (
	"encoding/json"
	"io"

	lint "github.com/Khaymah/dhow/apps/cli/internal/Lint"
)

const SchemaVersion = 1

type Report struct {
	SchemaVersion int           `json:"SchemaVersion"`
	File          string        `json:"File"`
	Lang          string        `json:"Lang"`
	Valid         bool          `json:"Valid"`
	Errors        int           `json:"Errors"`
	Warnings      int           `json:"Warnings"`
	Issues        []ReportIssue `json:"Issues"`
}

type ReportIssue struct {
	Code     string `json:"Code"`
	Severity string `json:"Severity"`
	Path     string `json:"Path"`
	Line     int    `json:"Line"`
	Column   int    `json:"Column"`
	Message  string `json:"Message"`
}

func NewReport(c Catalog, r lint.Result) Report {
	rep := Report{
		SchemaVersion: SchemaVersion, File: r.Path, Lang: c.Lang, Valid: r.Errors() == 0,
		Errors: r.Errors(), Warnings: r.Warnings(), Issues: make([]ReportIssue, 0, len(r.Issues)),
	}
	for _, i := range r.Issues {
		rep.Issues = append(rep.Issues, ReportIssue{
			Code: i.Code, Severity: string(i.Severity), Path: i.Path,
			Line: i.Position.Line, Column: i.Position.Column, Message: c.Issue(i),
		})
	}
	return rep
}

func JSON(w io.Writer, c Catalog, r lint.Result) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(NewReport(c, r))
}

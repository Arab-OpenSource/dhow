package lint

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Khaymah/dhow/internal/pipelinespec"
	"github.com/Khaymah/dhow/internal/safefile"
)

const DefaultFile = ".dhow.yml"

var (
	ErrNotFound   = errors.New("the file does not exist")
	ErrNotRegular = errors.New("the path is not a regular file")
	ErrSymlink    = errors.New("the path is a symbolic link")
	ErrNoAccess   = errors.New("permission to read the file is denied")
	ErrUnreadable = errors.New("the file could not be read")
)

type Result struct {
	Path   string
	Issues []pipelinespec.Issue
}

func (r Result) Errors() int {
	return r.count(pipelinespec.SeverityError)
}

func (r Result) Warnings() int {
	return r.count(pipelinespec.SeverityWarning)
}

func (r Result) count(s pipelinespec.Severity) int {
	n := 0
	for _, i := range r.Issues {
		if i.Severity == s {
			n++
		}
	}
	return n
}

func Resolve(path string) string {
	if path == "" {
		return DefaultFile
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return filepath.Join(path, DefaultFile)
	}
	return path
}

func File(path string) (Result, error) {
	src, err := read(path)
	if err != nil {
		return Result{Path: path}, err
	}
	return Source(path, src), nil
}

func Source(path string, src []byte) Result {
	_, issues := pipelinespec.Check(src)
	return Result{Path: path, Issues: issues}
}

func read(path string) ([]byte, error) {
	src, err := safefile.ReadRegular(path, pipelinespec.MaxFileBytes+1)
	if err != nil {
		return nil, classify(err)
	}
	return src, nil
}

func classify(err error) error {
	switch {
	case errors.Is(err, safefile.ErrSymlink):
		return ErrSymlink
	case errors.Is(err, safefile.ErrNotRegular):
		return ErrNotRegular
	case errors.Is(err, fs.ErrNotExist):
		return ErrNotFound
	case errors.Is(err, fs.ErrPermission):
		return ErrNoAccess
	default:
		return ErrUnreadable
	}
}

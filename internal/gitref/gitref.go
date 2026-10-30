package gitref

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxRefBytes     = 255
	maxNumberDigits = 10
	ShaBytesSHA1    = 40
	ShaBytesSHA256  = 64

	PrefixBranch       = "refs/heads/"
	PrefixTag          = "refs/tags/"
	prefixPull         = "refs/pull/"
	prefixMergeRequest = "refs/merge-requests/"
	suffixHead         = "/head"
)

type Kind uint8

const (
	KindBranch Kind = iota + 1
	KindTag
	KindPullRequest
	KindMergeRequest
)

func (k Kind) String() string {
	switch k {
	case KindBranch:
		return "branch"
	case KindTag:
		return "tag"
	case KindPullRequest:
		return "pull_request"
	case KindMergeRequest:
		return "merge_request"
	default:
		return ""
	}
}

var (
	ErrEmpty     = errors.New("the ref is empty")
	ErrTooLong   = errors.New("the ref is longer than 255 bytes")
	ErrForm      = errors.New("the ref is not refs/heads/<name>, refs/tags/<name>, refs/pull/<n>/head or refs/merge-requests/<n>/head")
	ErrName      = errors.New("the ref name breaks the rules of git check-ref-format")
	ErrCharacter = errors.New("the ref has a control, space, invisible or special character")
	ErrSha       = errors.New("a commit sha is 40 or 64 lowercase hexadecimal characters")
)

type Ref struct {
	Full   string
	Kind   Kind
	Name   string
	Number int
}

func Parse(full string) (Ref, error) {
	if full == "" {
		return Ref{}, ErrEmpty
	}
	if len(full) > MaxRefBytes {
		return Ref{}, ErrTooLong
	}
	if err := checkCharacters(full); err != nil {
		return Ref{}, err
	}
	switch {
	case strings.HasPrefix(full, PrefixBranch):
		return named(full, KindBranch, full[len(PrefixBranch):])
	case strings.HasPrefix(full, PrefixTag):
		return named(full, KindTag, full[len(PrefixTag):])
	case strings.HasPrefix(full, prefixPull):
		return numbered(full, KindPullRequest, full[len(prefixPull):])
	case strings.HasPrefix(full, prefixMergeRequest):
		return numbered(full, KindMergeRequest, full[len(prefixMergeRequest):])
	default:
		return Ref{}, ErrForm
	}
}

func Branch(name string) (Ref, error) { return Parse(PrefixBranch + name) }

func Tag(name string) (Ref, error) { return Parse(PrefixTag + name) }

func named(full string, kind Kind, name string) (Ref, error) {
	if err := CheckName(name); err != nil {
		return Ref{}, err
	}
	return Ref{Full: full, Kind: kind, Name: name}, nil
}

func numbered(full string, kind Kind, rest string) (Ref, error) {
	digits, ok := strings.CutSuffix(rest, suffixHead)
	if !ok || digits == "" || len(digits) > maxNumberDigits || digits[0] == '0' {
		return Ref{}, ErrForm
	}
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return Ref{}, ErrForm
		}
	}
	n, err := strconv.Atoi(digits)
	if err != nil || n < 1 {
		return Ref{}, ErrForm
	}
	return Ref{Full: full, Kind: kind, Name: digits, Number: n}, nil
}

func CheckName(name string) error {
	if name == "" {
		return ErrEmpty
	}
	if len(name) > MaxRefBytes {
		return ErrTooLong
	}
	if err := checkCharacters(name); err != nil {
		return err
	}
	if name == "@" || strings.Contains(name, "..") || strings.Contains(name, "@{") || strings.Contains(name, "//") ||
		strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".") {
		return ErrName
	}
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") || strings.HasPrefix(part, "-") || strings.HasSuffix(part, ".lock") {
			return ErrName
		}
	}
	return nil
}

func checkCharacters(v string) error {
	if !utf8.ValidString(v) {
		return ErrCharacter
	}
	for _, r := range v {
		if r <= ' ' || r == 0x7f || strings.ContainsRune(`~^:?*[\`, r) || unicode.IsControl(r) || invisible(r) {
			return ErrCharacter
		}
	}
	return nil
}

func invisible(r rune) bool {
	switch {
	case r >= 0x200b && r <= 0x200f, r >= 0x202a && r <= 0x202e, r >= 0x2060 && r <= 0x2069, r == 0xfeff, r == 0x061c:
		return true
	default:
		return unicode.Is(unicode.Cf, r) || unicode.IsSpace(r)
	}
}

func ValidSha(sha string) bool {
	if len(sha) != ShaBytesSHA1 && len(sha) != ShaBytesSHA256 {
		return false
	}
	for i := 0; i < len(sha); i++ {
		c := sha[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func CheckSha(sha string) error {
	if !ValidSha(sha) {
		return ErrSha
	}
	return nil
}

func IsZeroSha(sha string) bool {
	return ValidSha(sha) && strings.Trim(sha, "0") == ""
}

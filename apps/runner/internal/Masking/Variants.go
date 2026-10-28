package masking

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	bitsPerByte  = 8
	bitsPerChar  = 6
	base64Shifts = 3
)

var base64Alphabets = []*base64.Encoding{base64.StdEncoding, base64.URLEncoding}

func Variants(secret []byte) [][]byte {
	seen := map[string]bool{}
	var out [][]byte
	add := func(v []byte) {
		if len(v) < MinLength || seen[string(v)] {
			return
		}
		seen[string(v)] = true
		out = append(out, append([]byte(nil), v...))
	}
	forms := encodings
	if len(secret) > LongSecret || !utf8.Valid(secret) || bytes.IndexByte(secret, 0) >= 0 {
		forms = rawOnly
	}
	forms(secret, add)
	if bytes.ContainsAny(secret, "\r\n") {
		for _, line := range bytes.FieldsFunc(secret, func(r rune) bool { return r == '\n' || r == '\r' }) {
			forms(line, add)
		}
	}
	return out
}

func rawOnly(v []byte, add func([]byte)) { add(v) }

func encodings(v []byte, add func([]byte)) {
	if len(v) < MinLength {
		return
	}
	add(v)
	for _, enc := range base64Alphabets {
		add([]byte(enc.EncodeToString(v)))
		raw := enc.WithPadding(base64.NoPadding)
		add([]byte(raw.EncodeToString(v)))
		for shift := range base64Shifts {
			add(base64Window(raw, v, shift))
		}
	}
	wrapped(base64.StdEncoding.EncodeToString(v), add)
	add([]byte(url.QueryEscape(string(v))))
	add([]byte(url.PathEscape(string(v))))
	add([]byte(hex.EncodeToString(v)))
	add([]byte(strings.ToUpper(hex.EncodeToString(v))))
	for _, j := range jsonForms(v) {
		add(j)
	}
}

var wrapWidths = []int{64, 76}

const minWrappedTail = 8

func wrapped(encoded string, add func([]byte)) {
	for _, width := range wrapWidths {
		if len(encoded) <= width {
			continue
		}
		for i := 0; i < len(encoded); i += width {
			line := encoded[i:min(i+width, len(encoded))]
			if len(line) >= minWrappedTail {
				add([]byte(line))
			}
		}
	}
}

func base64Window(enc *base64.Encoding, v []byte, shift int) []byte {
	buf := make([]byte, shift+len(v))
	copy(buf[shift:], v)
	s := enc.EncodeToString(buf)
	start := (shift*bitsPerByte + bitsPerChar - 1) / bitsPerChar
	end := len(buf) * bitsPerByte / bitsPerChar
	if start >= end {
		return nil
	}
	return []byte(s[start:end])
}

func jsonForms(v []byte) [][]byte {
	var out [][]byte
	var html bytes.Buffer
	if err := json.NewEncoder(&html).Encode(string(v)); err == nil {
		out = append(out, unquote(html.Bytes()))
	}
	var plain bytes.Buffer
	enc := json.NewEncoder(&plain)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(string(v)); err == nil {
		p := unquote(plain.Bytes())
		out = append(out, p, bytes.ReplaceAll(p, []byte("/"), []byte(`\/`)))
	}
	out = append(out, asciiJSON(v))
	return out
}

func unquote(b []byte) []byte {
	b = bytes.TrimSuffix(b, []byte("\n"))
	if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
		return b[1 : len(b)-1]
	}
	return b
}

func asciiJSON(v []byte) []byte {
	var b strings.Builder
	for len(v) > 0 {
		r, size := utf8.DecodeRune(v)
		v = v[size:]
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == utf8.RuneError || r > 0x7e:
			writeUnicodeEscape(&b, r)
		default:
			b.WriteRune(r)
		}
	}
	return []byte(b.String())
}

func writeUnicodeEscape(b *strings.Builder, r rune) {
	if r > 0xffff {
		hi, lo := utf16.EncodeRune(r)
		fmt.Fprintf(b, `\u%04x\u%04x`, hi, lo)
		return
	}
	fmt.Fprintf(b, `\u%04x`, r)
}

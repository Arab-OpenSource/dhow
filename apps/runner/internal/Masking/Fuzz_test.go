package masking_test

import (
	"bytes"
	"testing"

	masking "github.com/Arab-OpenSource/dhow/apps/runner/internal/Masking"
)

func FuzzMasker(f *testing.F) {
	f.Add([]byte("s3cr3t-value"), []byte("prefix "), []byte(" suffix"), uint8(0), uint16(3))
	f.Add([]byte("aaaa"), []byte("aaa"), []byte("aaaaaaa"), uint8(2), uint16(1))
	f.Add([]byte("multi\nline\nsecret"), []byte("x"), []byte("\n"), uint8(9), uint16(7))
	f.Add([]byte("\x00\xff\xfe\x01"), []byte{0xff}, []byte{0}, uint8(4), uint16(2))
	f.Fuzz(func(t *testing.T, secret, prefix, suffix []byte, form uint8, split uint16) {
		if len(secret) > 512 || len(prefix) > 4096 || len(suffix) > 4096 || bytes.IndexByte(secret, '*') >= 0 {
			t.Skip()
		}
		m := masking.New()
		if !m.Add(secret) {
			return
		}
		variants := masking.Variants(secret)
		inject := variants[int(form)%len(variants)]
		data := append(append(append([]byte(nil), prefix...), inject...), suffix...)
		data = append(data, secret...)
		out := streamed(m, data, int(split))
		for _, v := range variants {
			if bytes.IndexByte(v, '*') >= 0 {
				continue
			}
			if bytes.Contains(out, v) {
				t.Fatalf("variant %q survived in %q", v, out)
			}
		}
		if !bytes.Contains(out, []byte(masking.Replacement)) {
			t.Fatalf("nothing was masked in %q", out)
		}
	})
}

func FuzzMaskerKeepsCleanText(f *testing.F) {
	f.Add([]byte("hello world\n"), []byte("not-in-text"), uint16(5))
	f.Add([]byte("\xe2\x82\xac\n\x00"), []byte("zzzz"), uint16(1))
	f.Fuzz(func(t *testing.T, data, secret []byte, split uint16) {
		if len(data) > 8192 || len(secret) > 256 {
			t.Skip()
		}
		m := masking.New()
		if !m.Add(secret) {
			return
		}
		for _, v := range masking.Variants(secret) {
			if bytes.Contains(data, v) {
				return
			}
		}
		if out := streamed(m, data, int(split)); !bytes.Equal(out, data) {
			t.Fatalf("clean text changed: %q became %q", data, out)
		}
	})
}

func streamed(m *masking.Masker, data []byte, split int) []byte {
	s := m.Stream()
	step := split%17 + 1
	var out []byte
	for len(data) > 0 {
		n := min(step, len(data))
		out = append(out, s.Write(data[:n])...)
		data = data[n:]
		step = step*7%13 + 1
	}
	return append(out, s.Flush()...)
}

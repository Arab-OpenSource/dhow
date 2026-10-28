package masking_test

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	masking "github.com/Khaymah/dhow/apps/runner/internal/Masking"
)

const secret = `p@ss/w0rd+"quoted"&<tag> ok`

func encodedForms(v string) map[string]string {
	b := []byte(v)
	return map[string]string{
		"raw":            v,
		"base64":         base64.StdEncoding.EncodeToString(b),
		"base64 raw":     base64.RawStdEncoding.EncodeToString(b),
		"base64 url":     base64.URLEncoding.EncodeToString(b),
		"base64 shift 1": base64.StdEncoding.EncodeToString(append([]byte("x"), b...)),
		"base64 shift 2": base64.StdEncoding.EncodeToString(append([]byte("xy"), b...)),
		"base64 url s1":  base64.RawURLEncoding.EncodeToString(append([]byte("Q"), b...)),
		"query":          url.QueryEscape(v),
		"path":           url.PathEscape(v),
		"hex":            hex.EncodeToString(b),
		"HEX":            strings.ToUpper(hex.EncodeToString(b)),
		"json":           `p@ss/w0rd+\"quoted\"&<tag> ok`,
		"json plain":     `p@ss/w0rd+\"quoted\"&<tag> ok`,
		"json slashes":   `p@ss\/w0rd+\"quoted\"&<tag> ok`,
	}
}

func TestEveryEncodingIsMasked(t *testing.T) {
	m := masking.New()
	require.True(t, m.Add([]byte(secret)))
	for name, form := range encodedForms(secret) {
		t.Run(name, func(t *testing.T) {
			out := m.MaskString("before " + form + " after\n")
			assert.NotContains(t, out, form)
			assert.Contains(t, out, masking.Replacement)
			assert.True(t, strings.HasPrefix(out, "before "), out)
			assert.True(t, strings.HasSuffix(out, " after\n") || strings.HasSuffix(out, "after\n"), out)
		})
	}
}

func TestBase64InsideALongerStreamIsMaskedAtEveryOffset(t *testing.T) {
	m := masking.New()
	require.True(t, m.Add([]byte(secret)))
	for prefix := range 3 {
		for suffix := range 3 {
			data := append(bytes.Repeat([]byte("A"), prefix), secret...)
			data = append(data, bytes.Repeat([]byte("B"), suffix)...)
			encoded := base64.StdEncoding.EncodeToString(data)
			out := m.MaskString(encoded)
			decoded, _ := base64.StdEncoding.DecodeString(strings.ReplaceAll(out, masking.Replacement, ""))
			assert.NotContains(t, string(decoded), "w0rd", "prefix %d suffix %d: %s", prefix, suffix, out)
			assert.Contains(t, out, masking.Replacement)
		}
	}
}

func TestMultiLineSecretsAreMaskedLineByLine(t *testing.T) {
	pem := "-----BEGIN TEST KEY-----\nMIIBVQIBADANBgkqhkiG9w0BAQEFAASC\nAT8wggE7AgEAAkEAq7BFUpkGp3+LQmlQ\n-----END TEST KEY-----"
	m := masking.New()
	require.True(t, m.Add([]byte(pem)))
	out := m.MaskString("key line: MIIBVQIBADANBgkqhkiG9w0BAQEFAASC\nAT8wggE7AgEAAkEAq7BFUpkGp3+LQmlQ\n")
	assert.NotContains(t, out, "MIIBVQIBADANBgkqhkiG9w0BAQEFAASC")
	assert.NotContains(t, out, "AT8wggE7AgEAAkEAq7BFUpkGp3+LQmlQ")
	assert.Contains(t, out, "key line: ")
}

func TestShortSecretsAreNotMasked(t *testing.T) {
	m := masking.New()
	assert.False(t, m.Add([]byte("abc")))
	assert.Equal(t, "abc abc", m.MaskString("abc abc"))
}

func TestOverlappingAndAdjacentMatchesAreAllCovered(t *testing.T) {
	m := masking.New()
	m.Add([]byte("abcdef"))
	m.Add([]byte("defghi"))
	assert.Equal(t, "x***y", m.MaskString("xabcdefghiy"))
	assert.Equal(t, "***-***", m.MaskString("abcdef-defghi"))
	assert.Equal(t, "***", m.MaskString("abcdefabcdef"))
}

func TestStreamHoldsBackOnlyAPossibleSecretPrefix(t *testing.T) {
	m := masking.New()
	m.Add([]byte("topsecret"))
	s := m.Stream()
	assert.Equal(t, "hello ", string(s.Write([]byte("hello top"))))
	assert.Equal(t, 3, s.Pending())
	assert.Equal(t, "***", string(s.Write([]byte("secret"))))
	assert.Equal(t, " tip", string(s.Write([]byte(" tip"))))
	assert.Empty(t, string(s.Write([]byte("tops"))))
	assert.Equal(t, "tops", string(s.Flush()))
}

func TestASecretSplitAcrossEveryBoundaryIsMasked(t *testing.T) {
	m := masking.New()
	m.Add([]byte(secret))
	data := "start " + secret + " middle " + base64.StdEncoding.EncodeToString([]byte(secret)) + " end"
	for cut := 1; cut < len(data); cut++ {
		s := m.Stream()
		out := string(s.Write([]byte(data[:cut]))) + string(s.Write([]byte(data[cut:]))) + string(s.Flush())
		assert.Equal(t, "start *** middle *** end", out, "cut %d", cut)
	}
}

func TestAddingASecretAppliesToBufferedOutput(t *testing.T) {
	m := masking.New()
	m.Add([]byte("first-secret"))
	s := m.Stream()
	assert.Equal(t, "line ", string(s.Write([]byte("line first-"))))
	m.Add([]byte("late-value"))
	out := string(s.Write([]byte("secret late-value\n"))) + string(s.Flush())
	assert.Equal(t, "*** ***\n", out)
}

func TestClearForgetsEverySecret(t *testing.T) {
	m := masking.New()
	m.Add([]byte(secret))
	m.Clear()
	assert.Equal(t, secret, m.MaskString(secret))
	assert.Zero(t, m.MaxPatternLength())
}

func TestTextWithoutSecretsPassesThroughUnchanged(t *testing.T) {
	m := masking.New()
	in := []byte("plain \x00\xff binary \xe2\x82 text\n")
	assert.Equal(t, in, m.Mask(in))
	m.Add([]byte("unrelated"))
	assert.Equal(t, in, m.Mask(in))
}

func TestVariantsIncludeTheDocumentedForms(t *testing.T) {
	got := map[string]bool{}
	for _, v := range masking.Variants([]byte("hunter2-é")) {
		got[string(v)] = true
		assert.GreaterOrEqual(t, len(v), masking.MinLength)
	}
	for _, want := range []string{
		"hunter2-é", "hunter2-%C3%A9", hex.EncodeToString([]byte("hunter2-é")), `hunter2-é`,
		base64.StdEncoding.EncodeToString([]byte("hunter2-é")),
	} {
		assert.True(t, got[want], want)
	}
}

func TestLargeSecretsStayCheap(t *testing.T) {
	sizes := []int{64 << 10, 256 << 10}
	for range 50 {
		sizes = append(sizes, 16<<10)
	}
	batch := make([][]byte, 0, len(sizes))
	for _, n := range sizes {
		b := make([]byte, n)
		_, err := rand.Read(b)
		require.NoError(t, err)
		batch = append(batch, b)
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	m := masking.New()
	added := m.AddAll(batch)
	took := time.Since(start)
	runtime.GC()
	runtime.ReadMemStats(&after)
	live := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	t.Logf("built in %s, %d MiB live", took, live>>20)
	assert.Less(t, took, 30*time.Second)
	assert.Less(t, live, int64(128<<20))
	for _, a := range added {
		assert.True(t, a)
	}
	out := m.Mask(append([]byte("x"), batch[1]...))
	assert.Equal(t, "x***", string(out), "a long secret is still masked raw")
	runtime.KeepAlive(m)
}

func TestLongOrBinarySecretsSkipTheEncodedForms(t *testing.T) {
	long := []byte(strings.Repeat("abcdefgh", masking.LongSecret/8+1))
	assert.Len(t, masking.Variants(long), 1)
	assert.Len(t, masking.Variants([]byte("bin\x00ary-value")), 1)
}

func TestWrappedBase64IsMasked(t *testing.T) {
	value := strings.Repeat("a long secret that wraps when encoded ", 3)
	m := masking.New()
	m.Add([]byte(value))
	encoded := base64.StdEncoding.EncodeToString([]byte(value))
	for _, width := range []int{64, 76} {
		var wrapped strings.Builder
		for i := 0; i < len(encoded); i += width {
			wrapped.WriteString(encoded[i:min(i+width, len(encoded))] + "\n")
		}
		out := m.MaskString(wrapped.String())
		for _, line := range strings.Split(strings.TrimSpace(wrapped.String()), "\n") {
			if len(line) >= 8 {
				assert.NotContains(t, out, line, "width %d", width)
			}
		}
	}
}

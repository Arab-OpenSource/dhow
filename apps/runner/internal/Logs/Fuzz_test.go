package logs

import (
	"bytes"
	"testing"
	"unicode/utf8"

	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

func FuzzChunker(f *testing.F) {
	f.Add([]byte("hello\nworld\n"), uint8(4))
	f.Add([]byte("داو منصة\n"), uint8(3))
	f.Add([]byte{0xe2, 0x82, 0xac, 0xff, 0xf0, 0x9f}, uint8(1))
	f.Fuzz(func(t *testing.T, data []byte, limit uint8) {
		n := int(limit)%64 + 1
		buf := append([]byte(nil), data...)
		var out [][]byte
		for len(buf) > 0 {
			cut := cutPoint(buf, n, true)
			if cut <= 0 || cut > n {
				t.Fatalf("cut %d outside 1..%d for %q", cut, n, buf)
			}
			out = append(out, buf[:cut])
			buf = buf[cut:]
		}
		if !bytes.Equal(bytes.Join(out, nil), data) {
			t.Fatalf("chunks do not join back to the input")
		}
		if utf8.Valid(data) && n >= utf8.UTFMax {
			for _, c := range out {
				if !utf8.Valid(c) {
					t.Fatalf("chunk %q splits a character", c)
				}
			}
		}
	})
}

type memStore struct{ b []byte }

func (m *memStore) WriteAt(p []byte, off int64) (int, error) {
	if need := int(off) + len(p); need > len(m.b) {
		m.b = append(m.b, make([]byte, need-len(m.b))...)
	}
	return copy(m.b[off:], p), nil
}

func (m *memStore) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(m.b)) {
		return 0, ErrSpillCorrupt
	}
	n := copy(p, m.b[off:])
	if n < len(p) {
		return n, ErrSpillCorrupt
	}
	return n, nil
}

func (m *memStore) Truncate(size int64) error { m.b = m.b[:size]; return nil }

func (m *memStore) Close() error { return nil }

func FuzzSpillRecords(f *testing.F) {
	f.Add([]byte("chunk one"), []byte("chunk two"), []byte{0, 0, 0, 9, 1, 2})
	f.Fuzz(func(t *testing.T, a, b, garbage []byte) {
		if len(a) == 0 || len(b) == 0 || len(a) > maxRecordBody/2 || len(b) > maxRecordBody/2 {
			t.Skip()
		}
		s := &spill{store: &memStore{}}
		for i, d := range [][]byte{a, b} {
			if err := s.push(&runnerv1.LogChunk{Seq: int64(i + 1), Stream: runnerv1.LogStream_LOG_STREAM_STDOUT, Data: d}); err != nil {
				t.Fatal(err)
			}
		}
		for i, want := range [][]byte{a, b} {
			c, err := s.pop()
			if err != nil || c.GetSeq() != int64(i+1) || !bytes.Equal(c.GetData(), want) {
				t.Fatalf("record %d did not round trip: %v", i, err)
			}
		}
		_, _, _ = decodeRecord(&memStore{b: garbage}, 0, maxRecordBody)
	})
}

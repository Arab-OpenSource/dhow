package logs

import (
	"bytes"
	"unicode/utf8"
)

func cutPoint(buf []byte, limit int, final bool) int {
	if len(buf) <= limit {
		if final {
			return len(buf)
		}
		return runeBoundary(buf, len(buf))
	}
	if i := bytes.LastIndexByte(buf[:limit], '\n'); i >= 0 {
		return i + 1
	}
	if n := runeBoundary(buf, limit); n > 0 {
		return n
	}
	return limit
}

func runeBoundary(buf []byte, end int) int {
	start := max(0, end-utf8.UTFMax+1)
	for i := end - 1; i >= start; i-- {
		b := buf[i]
		if b < utf8.RuneSelf {
			return end
		}
		if !utf8.RuneStart(b) {
			continue
		}
		size := runeLength(b)
		if size == 0 || i+size <= end {
			return end
		}
		return i
	}
	return end
}

func runeLength(lead byte) int {
	switch {
	case lead&0xE0 == 0xC0:
		return 2
	case lead&0xF0 == 0xE0:
		return 3
	case lead&0xF8 == 0xF0:
		return 4
	default:
		return 0
	}
}

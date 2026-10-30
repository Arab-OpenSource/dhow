package pipelinespec

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	errDuration = errors.New("invalid duration")
	errSize     = errors.New("invalid size")
	errCPU      = errors.New("invalid cpu")
)

func ParseDuration(s string) (time.Duration, error) {
	if s == "" || len(s) > 4*(MaxDurationDigits+1) {
		return 0, errDuration
	}
	units := []struct {
		suffix byte
		unit   time.Duration
	}{{'d', 24 * time.Hour}, {'h', time.Hour}, {'m', time.Minute}, {'s', time.Second}}
	var total time.Duration
	next := 0
	rest := s
	for rest != "" {
		end := 0
		for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
			end++
		}
		if end == 0 || end > MaxDurationDigits || end == len(rest) || (end > 1 && rest[0] == '0') {
			return 0, errDuration
		}
		n, err := strconv.ParseInt(rest[:end], 10, 64)
		if err != nil {
			return 0, errDuration
		}
		matched := false
		for next < len(units) {
			u := units[next]
			next++
			if rest[end] == u.suffix {
				total += time.Duration(n) * u.unit
				matched = true
				break
			}
		}
		if !matched {
			return 0, errDuration
		}
		rest = rest[end+1:]
	}
	return total, nil
}

func ParseSize(s string) (int64, error) {
	units := []struct {
		suffix string
		shift  uint
	}{{"Ki", 10}, {"Mi", 20}, {"Gi", 30}, {"Ti", 40}}
	for _, u := range units {
		digits, ok := strings.CutSuffix(s, u.suffix)
		if !ok {
			continue
		}
		n, ok := smallNumber(digits, 7)
		if !ok {
			return 0, errSize
		}
		return n << u.shift, nil
	}
	return 0, errSize
}

func ParseCPU(s Scalar) (int64, error) {
	text := s.Text
	if millis, ok := strings.CutSuffix(text, "m"); ok {
		n, ok := smallNumber(millis, 6)
		if !ok {
			return 0, errCPU
		}
		return n, nil
	}
	whole, frac, hasFrac := strings.Cut(text, ".")
	cores, ok := smallNumber(whole, 3)
	if !ok {
		return 0, errCPU
	}
	out := cores * 1000
	if hasFrac {
		if frac == "" || len(frac) > 3 {
			return 0, errCPU
		}
		f, ok := smallNumber(frac+strings.Repeat("0", 3-len(frac)), 3)
		if !ok {
			return 0, errCPU
		}
		out += f
	}
	return out, nil
}

func smallNumber(s string, maxDigits int) (int64, bool) {
	if s == "" || len(s) > maxDigits {
		return 0, false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}

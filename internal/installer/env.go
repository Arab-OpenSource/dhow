package installer

import (
	"bufio"
	"bytes"
	"strings"
)

type EnvEntry struct {
	Key   string
	Value string
}

func ParseEnv(data []byte) []EnvEntry {
	var out []EnvEntry
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out = append(out, EnvEntry{Key: strings.TrimSpace(k), Value: Unquote(strings.TrimSpace(v))})
	}
	return out
}

func EnvValue(entries []EnvEntry, key string) string {
	for _, e := range entries {
		if e.Key == key {
			return e.Value
		}
	}
	return ""
}

func RenderEnv(managed, existing []EnvEntry) []byte {
	var b bytes.Buffer
	seen := map[string]bool{}
	for _, e := range managed {
		seen[e.Key] = true
		b.WriteString(e.Key + "=" + e.Value + "\n")
	}
	for _, e := range existing {
		if seen[e.Key] {
			continue
		}
		seen[e.Key] = true
		b.WriteString(e.Key + "=" + e.Value + "\n")
	}
	return b.Bytes()
}

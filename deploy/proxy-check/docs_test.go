package proxycheck_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const guide = "../../docs/install/reverse-proxy.md"

var fixtures = []string{
	"nginx/dhow.conf",
	"caddy/Caddyfile",
	"traefik/compose.traefik.yaml",
}

var configLanguages = map[string]bool{"nginx": true, "caddyfile": true, "yaml": true}

var fence = regexp.MustCompile("(?ms)^```([a-z]*)\n(.*?)^```$")

type block struct {
	lang string
	body string
}

func guideBlocks(t *testing.T) []block {
	t.Helper()
	raw, err := os.ReadFile(guide)
	require.NoError(t, err)
	var out []block
	for _, m := range fence.FindAllStringSubmatch(string(raw), -1) {
		out = append(out, block{lang: m[1], body: m[2]})
	}
	require.NotEmpty(t, out, "no fenced code blocks in %s", guide)
	return out
}

func TestGuideShowsEveryTestedConfigVerbatim(t *testing.T) {
	blocks := guideBlocks(t)
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.FromSlash(name))
			require.NoError(t, err)
			found := false
			for _, b := range blocks {
				if b.body == string(want) {
					found = true
					break
				}
			}
			assert.True(t, found, "%s must contain deploy/proxy-check/%s byte for byte in a fenced block at the start of a line", guide, name)
		})
	}
}

func TestGuideShowsNoUntestedConfig(t *testing.T) {
	tested := make(map[string]bool, len(fixtures))
	for _, name := range fixtures {
		raw, err := os.ReadFile(filepath.FromSlash(name))
		require.NoError(t, err)
		tested[string(raw)] = true
	}
	for _, b := range guideBlocks(t) {
		if !configLanguages[b.lang] {
			continue
		}
		first, _, _ := strings.Cut(b.body, "\n")
		assert.True(t, tested[b.body], "%s has a %s block that run.sh does not test, starting with %q", guide, b.lang, first)
	}
}

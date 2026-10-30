package mptcp_test

import (
	"bufio"
	"bytes"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const directive = "multipathtcp=0"

func goModGodebug(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	require.NoError(t, err)
	var out []string
	inBlock := false
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case inBlock && line == ")":
			inBlock = false
		case inBlock:
			out = append(out, line)
		case line == "godebug (":
			inBlock = true
		case strings.HasPrefix(line, "godebug "):
			out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "godebug ")))
		}
	}
	require.NoError(t, sc.Err())
	return out
}

func TestGoModTurnsMPTCPOff(t *testing.T) {
	assert.Contains(t, goModGodebug(t), directive, "the root go.mod must keep `godebug multipathtcp=0` (CVE-2026-98432, ADR-0073)")
}

func TestTheBinaryCarriesTheSetting(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	require.True(t, ok)
	var defaults string
	for _, s := range info.Settings {
		if s.Key == "DefaultGODEBUG" {
			defaults = s.Value
		}
	}
	assert.Contains(t, strings.Split(defaults, ","), directive, "DefaultGODEBUG of this test binary")
}

func TestListenAndDialDefaultsAreOff(t *testing.T) {
	assert.False(t, (&net.ListenConfig{}).MultipathTCP(), "listeners")
	assert.False(t, (&net.Dialer{}).MultipathTCP(), "dialers")
}

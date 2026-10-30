package pipelinespec

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseCodes(src string) []string {
	_, issues := Parse([]byte(src))
	return issueCodes(issues, SeverityError)
}

func TestBillionLaughsIsRefusedQuickly(t *testing.T) {
	var b strings.Builder
	b.WriteString("a0: &a0 [x, x, x, x, x, x, x, x, x, x]\n")
	for i := 1; i < 10; i++ {
		b.WriteString("a")
		b.WriteString(string(rune('0' + i)))
		b.WriteString(": &a")
		b.WriteString(string(rune('0' + i)))
		b.WriteString(" [")
		for j := range 10 {
			if j > 0 {
				b.WriteString(", ")
			}
			b.WriteString("*a")
			b.WriteString(string(rune('0' + i - 1)))
		}
		b.WriteString("]\n")
	}
	start := time.Now()
	codes := parseCodes(b.String())
	assert.Less(t, time.Since(start), 2*time.Second)
	assert.Contains(t, codes, CodeAliasNotAllowed)
	assert.Contains(t, codes, CodeAnchorNotAllowed)
}

func TestParseRefusesNonCoreTags(t *testing.T) {
	for _, tag := range []string{"!custom", "!!binary", "!!timestamp", "!!set", "!<tag:example.com,2000:x>"} {
		assert.Contains(t, parseCodes("version: "+tag+" 1\n"), CodeTagNotAllowed, tag)
	}
	assert.NotContains(t, parseCodes("version: !!int 1\non: {push: {}}\n"), CodeTagNotAllowed)
}

func TestParseKeepsTimestampsAsText(t *testing.T) {
	spec, issues := Parse([]byte(runJob("") + "name: 2026-10-30\n"))
	require.NotNil(t, spec, "%v", issues)
	assert.Equal(t, "2026-10-30", spec.Name)
}

func TestParseNullTriggerMeansEnabled(t *testing.T) {
	spec, issues := Parse([]byte("version: 1\non:\n  pull_request:\n  api:\njobs: {}\n"))
	require.NotNil(t, spec, "%v", issues)
	assert.NotNil(t, spec.On.PullRequest)
	assert.NotNil(t, spec.On.API)
	assert.Nil(t, spec.On.Push)
}

func TestScalarKinds(t *testing.T) {
	spec, issues := Parse([]byte(job("    env: {A: 1, B: 1.5, C: true, D: x, E: 0x10, F:}\n    steps: [{run: x}]\n")))
	require.NotNil(t, spec, "%v", issues)
	env := spec.Jobs["a"].Env
	assert.Equal(t, KindInt, env["A"].Kind)
	assert.Equal(t, int64(16), env["E"].Int)
	assert.Equal(t, "0x10", env["E"].String())
	assert.Equal(t, KindFloat, env["B"].Kind)
	assert.Equal(t, KindBool, env["C"].Kind)
	assert.Equal(t, "x", env["D"].Value())
	assert.Empty(t, env["F"].Text)
}

func TestParseRefusesNullInLists(t *testing.T) {
	assert.Contains(t, parseCodes(job("    needs: [~]\n    steps: [{run: x}]\n")), CodeTypeMismatch)
}

func TestParseRefusesOutOfRangeInt(t *testing.T) {
	assert.Contains(t, parseCodes("version: 99999999999\n"), CodeTypeMismatch)
}

package pipelinespec

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func regexArtifactFlood(jobs int) string {
	vals := make([]string, 16)
	for i := range vals {
		vals[i] = fmt.Sprintf("v%d", i)
	}
	axis := "[" + strings.Join(vals, ", ") + "]"
	re := strings.Repeat(`(\\pL|\\pN|\\pM|\\pS)`, 90)
	var b strings.Builder
	b.WriteString("version: 1\non:\n  push: {}\njobs:\n")
	for j := range jobs {
		fmt.Fprintf(&b, "  j%d:\n    image: %s\n    strategy: {matrix: {a: %s, b: %s}}\n    steps: [{run: make}]\n    artifacts:\n", j, img, axis, axis)
		for k := range 16 {
			fmt.Fprintf(&b, "      - name: '${{ matrix.a.matches(\"%s\") ? \"x\" : matrix.a + matrix.b + \"-%d-%d\" }}'\n        paths: [out]\n", re, j, k)
		}
	}
	return b.String()
}

func TestRegexArtifactFloodFailsFast(t *testing.T) {
	src := regexArtifactFlood(8)
	start := time.Now()
	_, issues := Check([]byte(src))
	elapsed := time.Since(start)
	t.Logf("regex flood: %v", elapsed)
	assert.True(t, HasErrors(issues))
	assert.Less(t, elapsed, 200*time.Millisecond)
}

func TestArtifactNamesAreStaticTemplates(t *testing.T) {
	ok := job("    strategy: {matrix: {os: [a, b], v: ['1', '2']}}\n    steps: [{run: x}]\n" +
		"    artifacts: [{name: '${{ \"out-\" + matrix.os + \"-\" + matrix.v }}', paths: [x]}]\n")
	plan := compileSource(t, []byte(ok))
	names := []string{}
	for _, j := range plan.Jobs {
		names = append(names, j.Artifacts[0].Name)
	}
	assert.Equal(t, []string{"out-a-1", "out-a-2", "out-b-1", "out-b-2"}, names)

	for _, expr := range []string{
		`matrix.os.startsWith("a") ? "x" : matrix.os`, `"out-" + string(1)`, `[matrix.os][0]`, `matrix.os + matrix.os.substring(0)`,
	} {
		src := job("    strategy: {matrix: {os: [a, b]}}\n    steps: [{run: x}]\n    artifacts: [{name: '${{ " + expr + " }}', paths: [x]}]\n")
		_, issues := Check([]byte(src))
		codes := issueCodes(issues, SeverityError)
		assert.NotEmpty(t, codes, expr)
	}
}

func TestExpressionBudget(t *testing.T) {
	var b strings.Builder
	for j := range 11 {
		fmt.Fprintf(&b, "  j%d:\n    image: %s\n    steps:\n      - run: x\n        env:\n", j, img)
		for e := range 99 {
			fmt.Fprintf(&b, "          E%d: ${{ git.branch }}\n", e)
		}
	}
	_, issues := Check([]byte(pipeline(b.String())))
	assert.Contains(t, issueCodes(issues, SeverityError), CodeExpressionBudget)
}

func longKeyFile() string {
	var b strings.Builder
	key := strings.Repeat("k", 1000)
	for i := range 30 {
		b.WriteString("{" + key + string(rune('a'+i%26)) + ": ")
	}
	b.WriteString("{")
	for b.Len() < MaxFileBytes-200 {
		b.WriteString("a: 0,")
	}
	b.WriteString("a: 0}")
	for range 30 {
		b.WriteString("}")
	}
	return b.String()
}

func allocatedBy(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func TestLongKeysDoNotExhaustMemory(t *testing.T) {
	src := []byte(longKeyFile())
	parser := allocatedBy(func() {
		var n yaml.Node
		_ = yaml.Unmarshal(src, &n)
	})
	var issues []Issue
	start := time.Now()
	total := allocatedBy(func() { _, issues = Check(src) })
	t.Logf("long keys: %d bytes in Check, %d of them in the YAML parser, %v", total, parser, time.Since(start))
	assert.Contains(t, issueCodes(issues, SeverityError), CodeKeyTooLong)
	assert.Less(t, total, parser+4<<20, "Check allocated %d bytes beyond the parser", total-parser)
	for _, i := range issues {
		assert.LessOrEqual(t, len(i.Path), MaxPathBytes)
	}
}

func TestIssuesStopAtTheCap(t *testing.T) {
	var b strings.Builder
	b.WriteString("version: 1\non: {push: {}}\njobs: {}\nx:\n")
	for i := range 5000 {
		fmt.Fprintf(&b, "  - &a%d 1\n", i)
	}
	_, issues := Parse([]byte(b.String()))
	assert.Len(t, issues, MaxIssues)
}

func TestPathsAreCapped(t *testing.T) {
	p := joinPath(strings.Repeat("a", 400), strings.Repeat("ب", 200))
	assert.LessOrEqual(t, len(p), MaxPathBytes)
	assert.True(t, strings.HasSuffix(p, pathEllipsis))
}

func TestSecretBackedEnvIsNotReadable(t *testing.T) {
	src := job("    env:\n      TOKEN: ${{ secrets.TOKEN }}\n    steps:\n" +
		"      - run: x\n        if: '${{ !(\"TOKEN\" in env) && !(\"OTHER\" in env) && env.PLAIN == \"p\" }}'\n" +
		"        env:\n          OUT: '${{ \"TOKEN\" in env ? env.TOKEN : \"none\" }}'\n" +
		"    cache: {key: '${{ \"TOKEN\" in env ? env.TOKEN : \"none\" }}', paths: [x]}\n")
	plan := compileSource(t, []byte(src))
	r := plan.Jobs[0]
	c := Context{
		Env:       map[string]string{"TOKEN": "s3cret", "OTHER": "s3cret2", "PLAIN": "p"},
		SecretEnv: []string{"OTHER"},
	}
	ok, err := r.EvalStepIf(context.Background(), 0, c)
	require.NoError(t, err)
	assert.True(t, ok)
	out, err := r.EvalValue(context.Background(), "Steps[0].Env.OUT", c)
	require.NoError(t, err)
	assert.Equal(t, "none", out)
	d := r.decls
	secret := map[string]bool{"TOKEN": true, "OTHER": true}
	act, err := activation(c, d, r.combo, secret)
	require.NoError(t, err)
	cacheOut, err := evaluate(context.Background(), scopeStepValue, d, r.Cache.Key.Expression, act)
	require.NoError(t, err)
	assert.Equal(t, "none", cacheOut)
}

func TestContextIsBounded(t *testing.T) {
	plan := compileSource(t, []byte(job("    if: ${{ git.branch == \"main\" }}\n    steps: [{run: x}]\n")))
	r := plan.Jobs[0]
	cases := []Context{
		{Git: Git{Message: strings.Repeat("m", MaxMessageBytes+1)}},
		{Git: Git{Branch: strings.Repeat("b", MaxContextTextBytes+1)}},
		{Vars: map[string]string{"V": strings.Repeat("v", MaxContextVarBytes+1)}},
	}
	for i, c := range cases {
		_, err := r.EvalIf(context.Background(), c)
		require.ErrorIs(t, err, ErrContextInvalid, "case %d", i)
	}
}

func TestEvaluatedValuesAreBounded(t *testing.T) {
	plan := compileSource(t, []byte(job("    env: {A: '${{ git.message + \"x\" }}'}\n    steps: [{run: x}]\n")))
	_, err := plan.Jobs[0].EvalValue(context.Background(), "Env.A", Context{Git: Git{Message: strings.Repeat("m", MaxMessageBytes)}})
	require.ErrorIs(t, err, ErrExpressionResult)
}

func TestRunJobKeysCannotCollide(t *testing.T) {
	a := combination{{Key: "a", Value: Scalar{Kind: KindString, Text: "x, b=y"}}, {Key: "b", Value: Scalar{Kind: KindString, Text: "z"}}}
	b := combination{{Key: "a", Value: Scalar{Kind: KindString, Text: "x"}}, {Key: "b", Value: Scalar{Kind: KindString, Text: "y, b=z"}}}
	assert.NotEqual(t, runJobKey("j", a), runJobKey("j", b))
	keys := matrixKeys(t, `{a: ["x, b=y", x], b: [z, "y, b=z"]}`)
	assert.Len(t, keys, 4)
	assert.Contains(t, keys, `a (a=x\, b\=y, b=z)`)
}

package pipelinespec

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func numbers(n int) string {
	parts := make([]string, 0, n)
	for i := range n {
		parts = append(parts, string(rune('0'+i%10)))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func expensiveCondition() string {
	l := numbers(60)
	return l + ".all(a, " + l + ".all(b, " + l + ".all(c, " + l + ".all(d, a + b + c + d >= 0))))"
}

func TestExpensiveExpressionIsRefusedAtValidation(t *testing.T) {
	cond := expensiveCondition()
	require.Less(t, len(cond), MaxExpressionBytes)
	_, issues := Check([]byte(job("    if: ${{ " + cond + " }}\n    steps: [{run: x}]\n")))
	assert.Contains(t, issueCodes(issues, SeverityError), CodeExpressionTooExpensive)
}

func TestRuntimeLimitsStillStopAnExpensiveExpression(t *testing.T) {
	p, err := prepare(scopeJobIf, &declarations{}, expensiveCondition())
	require.NoError(t, err)
	for range 3 {
		start := time.Now()
		_, err = p.run(context.Background(), map[string]any{}, ExpressionTimeout)
		elapsed := time.Since(start)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrExpressionCost) || errors.Is(err, ErrExpressionTimeout), "%v", err)
		assert.Less(t, elapsed, 50*time.Millisecond)
	}
}

func TestCostLimitStopsBeforeTheTimeout(t *testing.T) {
	p, err := prepare(scopeJobIf, &declarations{}, expensiveCondition())
	require.NoError(t, err)
	_, err = p.run(context.Background(), map[string]any{}, 10*time.Second)
	require.ErrorIs(t, err, ErrExpressionCost)
}

func TestExpressionStopsWhenTheContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := &declarations{}
	_, err := evaluate(ctx, scopeJobIf, d, numbers(60)+".all(a, "+numbers(60)+".all(b, a >= 0))", map[string]any{})
	require.ErrorIs(t, err, ErrExpressionTimeout)
}

func TestTypeErrorsAreCaughtAtValidation(t *testing.T) {
	for _, cond := range []string{`git.branch`, `git.pr.number == "1"`, `git.is_tag + 1`, `size(git.ref)`, `vars.x == 1`} {
		_, issues := Check([]byte(job("    if: ${{ " + cond + " }}\n    steps: [{run: x}]\n")))
		assert.Contains(t, issueCodes(issues, SeverityError), CodeExpressionType, cond)
	}
}

func TestRegexAndTimeFunctionsAreNotAvailable(t *testing.T) {
	for _, cond := range []string{
		`git.message.matches("a+")`, `matches(git.message, "a")`, `timestamp("2026-10-30T00:00:00Z") != timestamp("2026-10-31T00:00:00Z")`,
		`duration("1h") == duration("1h")`,
	} {
		_, issues := Check([]byte(job("    if: ${{ " + cond + " }}\n    steps: [{run: x}]\n")))
		codes := issueCodes(issues, SeverityError)
		assert.True(t, slices.Contains(codes, CodeExpressionUnknownName) || slices.Contains(codes, CodeExpressionType), "%s: %v", cond, codes)
	}
}

func TestEvaluation(t *testing.T) {
	src := `version: 1
on:
  manual:
    inputs:
      force: {type: boolean}
concurrency:
  group: ${{ "deploy-" + git.branch }}
jobs:
  a:
    image: ` + img + `
    steps: [{run: x}]
  b:
    needs: [a]
    image: ` + img + `
    strategy:
      matrix: {os: [debian, ubuntu]}
    if: ${{ (git.branch == "main" || inputs.force) && needs.a.outputs.ok == "yes" }}
    env:
      TARGET: ${{ project.name + "-" + matrix.os }}
    steps:
      - run: deploy
        if: ${{ env.MODE == "full" && !git.pr.is_fork }}
        env:
          MESSAGE: ${{ git.message }}
`
	plan := compileSource(t, []byte(src))
	ctx := context.Background()
	c := Context{
		Git:     Git{Branch: "main", Message: "$(touch /tmp/x) ${{ secrets.T }}"},
		Project: Project{Name: "web"},
		Needs:   map[string]any{"a": map[string]string{"ok": "yes"}},
		Env:     map[string]string{"MODE": "full"},
	}
	group, err := plan.EvalGroup(ctx, c)
	require.NoError(t, err)
	assert.Equal(t, "deploy-main", group)

	b := jobByKey(t, plan, "b (os=ubuntu)")
	ok, err := b.EvalIf(ctx, c)
	require.NoError(t, err)
	assert.True(t, ok)
	target, err := b.EvalValue(ctx, "Env.TARGET", c)
	require.NoError(t, err)
	assert.Equal(t, "web-ubuntu", target)
	msg, err := b.EvalValue(ctx, "Steps[0].Env.MESSAGE", c)
	require.NoError(t, err)
	assert.Equal(t, c.Git.Message, msg)
	stepOK, err := b.EvalStepIf(ctx, 0, c)
	require.NoError(t, err)
	assert.True(t, stepOK)

	c.Git.Branch = "dev"
	ok, err = b.EvalIf(ctx, c)
	require.NoError(t, err)
	assert.False(t, ok)
	c.Inputs = map[string]any{"force": true}
	ok, err = b.EvalIf(ctx, c)
	require.NoError(t, err)
	assert.True(t, ok)

	_, err = b.EvalValue(ctx, "Env.MISSING", c)
	require.ErrorIs(t, err, ErrExpression)
	_, err = b.EvalStepIf(ctx, 5, c)
	require.ErrorIs(t, err, ErrExpression)
	a := jobByKey(t, plan, "a")
	ok, err = a.EvalIf(ctx, c)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestEvaluationRefusesWrongRuntimeType(t *testing.T) {
	plan := compileSource(t, []byte(job("    strategy: {matrix: {v: [1, x]}}\n    env: {A: '${{ matrix.v }}'}\n    steps: [{run: x}]\n")))
	_, err := jobByKey(t, plan, "a (v=1)").EvalValue(context.Background(), "Env.A", Context{})
	require.ErrorIs(t, err, ErrExpressionResult)
	v, err := jobByKey(t, plan, "a (v=x)").EvalValue(context.Background(), "Env.A", Context{})
	require.NoError(t, err)
	assert.Equal(t, "x", v)
}

func TestSplitExpression(t *testing.T) {
	cases := []struct {
		in, out string
		expr    bool
		err     bool
	}{
		{"plain", "plain", false, false},
		{"${{ a }}", "a", true, false},
		{"  ${{a}}  ", "a", true, false},
		{"x${{ a }}", "", false, true},
		{"${{ a }}x", "", false, true},
		{"${{ a }} ${{ b }}", "", false, true},
		{"${{ }}", "", false, true},
		{"${{ a", "", false, true},
	}
	for _, tc := range cases {
		out, expr, err := splitExpression(tc.in)
		assert.Equal(t, tc.err, err != nil, tc.in)
		assert.Equal(t, tc.expr, expr, tc.in)
		assert.Equal(t, tc.out, out, tc.in)
	}
}

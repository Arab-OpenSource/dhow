package pipelinespec

import (
	"context"
	"errors"
	"testing"

	"github.com/google/cel-go/cel"
)

func FuzzParse(f *testing.F) {
	f.Add([]byte(runJob("")))
	f.Add([]byte("version: 1\non: {push: {}}\njobs: {a: {image: &x a, steps: [*x]}}\n"))
	f.Add([]byte("a: !!binary aGk=\n<<: {b: 1}\n"))
	f.Add([]byte(job("    strategy: {matrix: {os: [a, b], include: [{os: c}]}}\n    if: ${{ matrix.os == \"a\" }}\n    steps: [{run: x}]\n")))
	f.Add([]byte(job("    env: {A: '${{ secrets.T }}'}\n    steps: [{run: x}]\n    cache: {key: '${{ hash_files(\"a\") }}', paths: [x]}\n")))
	f.Fuzz(func(t *testing.T, src []byte) {
		spec, issues := Check(src)
		for _, i := range issues {
			if i.Code == "" || i.Message == "" {
				t.Fatalf("issue without code or message: %+v", i)
			}
		}
		if spec == nil || HasErrors(issues) {
			return
		}
		first, err := Compile(spec)
		if err != nil {
			return
		}
		if len(first.Jobs) > MaxRunJobs {
			t.Fatalf("plan has %d jobs", len(first.Jobs))
		}
		again, err := Compile(spec)
		if err != nil {
			t.Fatalf("second compile failed: %v", err)
		}
		for i := range first.Jobs {
			if first.Jobs[i].Digest != again.Jobs[i].Digest {
				t.Fatalf("digest changed for %s", first.Jobs[i].Key)
			}
		}
	})
}

func FuzzCEL(f *testing.F) {
	for _, s := range []string{
		`git.branch == "main"`, `git.is_tag && git.tag.startsWith("v")`, `git.message.matches("(a+)+$")`,
		`[1,2,3].all(x, [1,2,3].exists(y, x == y))`, `vars.x + "-" + project.name`, `secrets.TOKEN`,
		`matrix.os == "a" || inputs.flag`, `size(needs.build.outputs) > 0`, `1/0 == 1`, `"a" * 3`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		d := &declarations{
			inputs: map[string]*cel.Type{"flag": cel.BoolType},
			matrix: map[string]*cel.Type{"os": cel.StringType},
			needs:  map[string]bool{"build": false},
		}
		for _, scope := range []exprScope{scopeJobIf, scopeJobValue, scopeCacheKey} {
			issues := &issueList{positions: map[string]Position{}}
			c := newExprChecker(issues, d)
			v, ok := c.check(scope, src, "x")
			if !ok || v.expression == "" || scope == scopeCacheKey {
				continue
			}
			act := activation(Context{Inputs: map[string]any{"flag": true}}, d, combination{{Key: "os", Value: Scalar{Kind: KindString, Text: "a"}}})
			_, err := evaluate(context.Background(), scope, d, v.expression, act)
			if err != nil && !knownEvalError(err) {
				t.Fatalf("unexpected error type: %v", err)
			}
		}
	})
}

func knownEvalError(err error) bool {
	for _, known := range []error{ErrExpression, ErrExpressionResult, ErrExpressionCost, ErrExpressionTimeout, ErrExpressionEval} {
		if errors.Is(err, known) {
			return true
		}
	}
	return false
}

func FuzzMatrix(f *testing.F) {
	for _, s := range []string{
		"{os: [a, b], node: [1, 2]}", "{os: [a, b], exclude: [{os: a}]}", "{include: [{a: 1}, {a: 2}]}",
		"{a: [1,2,3,4,5,6,7,8], b: [1,2,3,4,5,6,7,8], c: [1,2,3,4,5]}", "{a: [x, x]}", "{a: []}", "{a: [1, '1', 1.0, true]}",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, matrix string) {
		src := job("    strategy:\n      matrix: " + matrix + "\n    steps: [{run: x}]\n")
		spec, issues := Check([]byte(src))
		if spec == nil || HasErrors(issues) {
			return
		}
		plan, err := Compile(spec)
		if err != nil {
			return
		}
		if len(plan.Jobs) == 0 || len(plan.Jobs) > MaxMatrixJobs {
			t.Fatalf("matrix expanded to %d jobs", len(plan.Jobs))
		}
		seen := map[string]bool{}
		for _, j := range plan.Jobs {
			if seen[j.Key] {
				t.Fatalf("duplicate key %q", j.Key)
			}
			seen[j.Key] = true
		}
	})
}

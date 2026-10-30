package pipelinespec

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidPipelinesPass(t *testing.T) {
	cases := map[string]string{
		"minimal":            runJob(""),
		"bare if":            job("    if: git.branch == \"main\"\n    steps: [{run: make}]\n"),
		"null trigger":       "version: 1\non:\n  pull_request:\njobs:\n  a:\n    image: " + img + "\n    steps: [{run: make}]\n",
		"plugin only job":    pipeline("  a:\n    steps:\n      - uses: ghcr.io/x/y@sha256:" + digest + "\n"),
		"include only":       job("    strategy:\n      matrix:\n        include: [{os: a}, {os: b}]\n    steps: [{run: make}]\n"),
		"needs outputs":      pipeline("  a:\n    image: " + img + "\n    steps: [{run: x}]\n  b:\n    needs: [a]\n    if: ${{ needs.a.outputs.v == \"1\" }}\n    image: " + img + "\n    steps: [{run: x}]\n"),
		"step env reads env": job("    env: {A: x}\n    steps:\n      - run: make\n        env:\n          B: ${{ env.A + \"-b\" }}\n"),
		"input string":       "version: 1\non:\n  manual:\n    inputs:\n      target: {type: choice, options: [a, b], default: a}\njobs:\n  a:\n    image: " + img + "\n    if: ${{ inputs.target == \"a\" }}\n    steps: [{run: make}]\n",
		"numbers as env":     job("    env: {PORT: 8080, ON: true}\n    steps: [{run: make}]\n"),
		"tag image warns":    pipeline("  a:\n    image: alpine:3\n    steps: [{run: make}]\n"),
		"retention":          job("    steps: [{run: make}]\n    artifacts: [{name: x, paths: [out], retention: 7d}]\n"),
		"resources":          job("    resources: {cpu: 1500m, memory: 512Mi, disk: 10Gi, pids: 64}\n    steps: [{run: make}]\n"),
		"matrix artifact":    job("    strategy:\n      matrix: {os: [a, b]}\n    steps: [{run: make}]\n    artifacts: [{name: '${{ \"out-\" + matrix.os }}', paths: [out]}]\n"),
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			spec, issues := Check([]byte(src))
			require.NotNil(t, spec, "%v", issues)
			assert.Empty(t, issueCodes(issues, SeverityError), "%v", issues)
		})
	}
}

type errorCase struct {
	name string
	src  string
	code string
}

func structureErrorCases() []errorCase {
	return []errorCase{
		{"cycle", pipeline("  a:\n    needs: [b]\n    image: " + img + "\n    steps: [{run: x}]\n  b:\n    needs: [a]\n    image: " + img + "\n    steps: [{run: x}]\n"), CodeDAGCycle},
		{"self need", job("    needs: [a]\n    steps: [{run: x}]\n"), CodeDAGCycle},
		{"unknown need", job("    needs: [ghost]\n    steps: [{run: x}]\n"), CodeNeedUnknown},
		{"duplicate need", pipeline("  a:\n    image: " + img + "\n    steps: [{run: x}]\n  b:\n    needs: [a, a]\n    image: " + img + "\n    steps: [{run: x}]\n"), CodeNeedDuplicate},
		{"unknown key", job("    stepz: []\n    steps: [{run: x}]\n"), CodeUnknownKey},
		{"unknown top key", "version: 1\nservices: {}\non: {push: {}}\njobs: {a: {image: " + img + ", steps: [{run: x}]}}\n", CodeUnknownKey},
		{"alias", "version: 1\non: {push: {}}\njobs:\n  a:\n    image: &i " + img + "\n    steps: [{run: x}]\n  b:\n    image: *i\n    steps: [{run: x}]\n", CodeAliasNotAllowed},
		{"anchor", job("    env: &e {A: b}\n    steps: [{run: x}]\n"), CodeAnchorNotAllowed},
		{"merge key", job("    env:\n      <<: {A: b}\n    steps: [{run: x}]\n"), CodeMergeKeyNotAllowed},
		{"custom tag", job("    env: {A: !secret x}\n    steps: [{run: x}]\n"), CodeTagNotAllowed},
		{"binary tag", job("    env: {A: !!binary aGk=}\n    steps: [{run: x}]\n"), CodeTagNotAllowed},
		{"duplicate key", job("    steps: [{run: x}]\n    steps: [{run: y}]\n"), CodeDuplicateKey},
		{"duplicate job", "version: 1\non: {push: {}}\njobs:\n  a: {image: " + img + ", steps: [{run: x}]}\n  a: {image: " + img + ", steps: [{run: x}]}\n", CodeDuplicateKey},
		{"expression in run", runJob("      - run: echo ${{ git.message }}\n"), CodeRunExpression},
		{"expression in run no spaces", runJob("      - run: echo ${{git.message}}\n"), CodeRunExpression},
		{"secrets in job if", job("    if: ${{ secrets.TOKEN == \"x\" }}\n    steps: [{run: x}]\n"), CodeSecretsInIf},
		{"secrets in step if", runJob("      - run: x\n        if: secrets.TOKEN != \"\"\n"), CodeSecretsInIf},
		{"secret in larger expression", job("    env: {A: '${{ \"x\" + secrets.T }}'}\n    steps: [{run: x}]\n"), CodeSecretNotWholeValue},
		{"secret in group", "version: 1\non: {push: {}}\nconcurrency: {group: '${{ secrets.T }}'}\njobs: {a: {image: " + img + ", steps: [{run: x}]}}\n", CodeSecretsNotAllowed},
		{"secret in cache key", job("    steps: [{run: x}]\n    cache: {key: '${{ secrets.T }}', paths: [x]}\n"), CodeSecretsNotAllowed},
		{"matrix too large", job("    strategy:\n      matrix:\n        a: [1,2,3,4,5,6,7,8]\n        b: [1,2,3,4,5,6,7,8]\n        c: [1,2,3,4,5]\n    steps: [{run: x}]\n"), CodeMatrixTooLarge},
		{"matrix raw too large", job("    strategy:\n      matrix:\n        a: [1,2,3,4,5,6,7,8]\n        b: [1,2,3,4,5,6,7,8]\n        c: [1,2,3,4,5,6,7,8]\n        d: [1,2,3,4,5,6,7,8]\n    steps: [{run: x}]\n"), CodeMatrixTooLarge},
		{"uses without digest", runJob("      - uses: ghcr.io/x/y:1\n"), CodeUsesNotPinned},
		{"uses latest", runJob("      - uses: ghcr.io/x/y\n"), CodeUsesNotPinned},
		{"uses invalid", runJob("      - uses: 'not an image'\n"), CodeUsesInvalid},
		{"version", "version: 2\non: {push: {}}\njobs: {a: {image: " + img + ", steps: [{run: x}]}}\n", CodeVersionUnsupported},
		{"missing version", "on: {push: {}}\njobs: {a: {image: " + img + ", steps: [{run: x}]}}\n", CodeVersionUnsupported},
		{"no triggers", "version: 1\njobs: {a: {image: " + img + ", steps: [{run: x}]}}\n", CodeTriggersRequired},
		{"schedule", "version: 1\non: {schedule: [{cron: '* * * * *'}]}\njobs: {a: {image: " + img + ", steps: [{run: x}]}}\n", CodeTriggerNotSupported},
		{"artifact trigger", "version: 1\non: {artifact: {from: x}}\njobs: {a: {image: " + img + ", steps: [{run: x}]}}\n", CodeTriggerNotSupported},
		{"no jobs", "version: 1\non: {push: {}}\n", CodeJobsRequired},
		{"bad job id", pipeline("  Build:\n    image: " + img + "\n    steps: [{run: x}]\n"), CodeJobIDInvalid},
		{"no steps", job("    steps: []\n"), CodeStepsRequired},
		{"run and uses", runJob("      - run: x\n        uses: ghcr.io/x/y@sha256:" + digest + "\n"), CodeStepRunOrUses},
		{"neither run nor uses", runJob("      - name: nothing\n"), CodeStepRunOrUses},
		{"with without uses", runJob("      - run: x\n        with: {a: b}\n"), CodeWithWithoutUses},
		{"image required", pipeline("  a:\n    steps: [{run: x}]\n"), CodeImageRequired},
		{"image expression", pipeline("  a:\n    image: '${{ matrix.i }}'\n    steps: [{run: x}]\n"), CodeExpressionNotAllowed},
		{"env reserved", job("    env: {DHOW_X: a}\n    steps: [{run: x}]\n"), CodeEnvNameReserved},
		{"env name", job("    env: {'1A': a}\n    steps: [{run: x}]\n"), CodeEnvNameInvalid},
		{"partial expression", job("    env: {A: 'v${{ git.tag }}'}\n    steps: [{run: x}]\n"), CodeExpressionSyntax},
		{"cel syntax", job("    if: ${{ git.branch == }}\n    steps: [{run: x}]\n"), CodeExpressionSyntax},
		{"if not bool", job("    if: ${{ git.branch }}\n    steps: [{run: x}]\n"), CodeExpressionType},
		{"env not string", job("    env: {A: '${{ git.is_tag }}'}\n    steps: [{run: x}]\n"), CodeExpressionType},
		{"bad comparison", job("    if: ${{ git.branch == 1 }}\n    steps: [{run: x}]\n"), CodeExpressionType},
	}
}

func policyErrorCases() []errorCase {
	return []errorCase{
		{"unknown variable", job("    if: ${{ github.ref == \"x\" }}\n    steps: [{run: x}]\n"), CodeExpressionUnknownName},
		{"undeclared input", job("    if: ${{ inputs.x == \"y\" }}\n    steps: [{run: x}]\n"), CodeExpressionUnknownName},
		{"needs not listed", pipeline("  a:\n    image: " + img + "\n    steps: [{run: x}]\n  b:\n    if: ${{ needs.a.outputs.v == \"1\" }}\n    image: " + img + "\n    steps: [{run: x}]\n"), CodeExpressionUnknownName},
		{"env in job if", job("    if: ${{ env.A == \"1\" }}\n    steps: [{run: x}]\n"), CodeExpressionUnknownName},
		{"hash_files outside cache", job("    env: {A: '${{ hash_files(\"x\") }}'}\n    steps: [{run: x}]\n"), CodeExpressionUnknownName},
		{"long expression", job("    if: ${{ " + strings.Repeat("true && ", 300) + "true }}\n    steps: [{run: x}]\n"), CodeExpressionTooLong},
		{"bad timeout", job("    timeout: 2y\n    steps: [{run: x}]\n"), CodeTimeoutInvalid},
		{"timeout too long", job("    timeout: 25h\n    steps: [{run: x}]\n"), CodeTimeoutInvalid},
		{"step longer than job", job("    timeout: 10m\n    steps: [{run: x, timeout: 11m}]\n"), CodeTimeoutInvalid},
		{"cpu", job("    resources: {cpu: 100}\n    steps: [{run: x}]\n"), CodeResourceInvalid},
		{"memory unit", job("    resources: {memory: 4GB}\n    steps: [{run: x}]\n"), CodeResourceInvalid},
		{"egress ip", job("    network: {egress: [10.0.0.1]}\n    steps: [{run: x}]\n"), CodeEgressInvalid},
		{"label", job("    runner: {labels: [Bad Label]}\n    steps: [{run: x}]\n"), CodeLabelInvalid},
		{"gvisor", job("    runner: {isolation: gvisor}\n    steps: [{run: x}]\n"), CodeIsolationNotSupported},
		{"microvm", job("    runner: {isolation: microvm}\n    steps: [{run: x}]\n"), CodeIsolationInvalid},
		{"pattern", "version: 1\non: {push: {branches: ['feat/[a-z]']}}\njobs: {a: {image: " + img + ", steps: [{run: x}]}}\n", CodePatternInvalid},
		{"pattern stars", "version: 1\non: {push: {branches: ['a**']}}\njobs: {a: {image: " + img + ", steps: [{run: x}]}}\n", CodePatternInvalid},
		{"choice default", "version: 1\non: {manual: {inputs: {t: {type: choice, options: [a], default: b}}}}\njobs: {a: {image: " + img + ", steps: [{run: x}]}}\n", CodeInputInvalid},
		{"boolean default", "version: 1\non: {manual: {inputs: {t: {type: boolean, default: yes}}}}\njobs: {a: {image: " + img + ", steps: [{run: x}]}}\n", CodeInputInvalid},
		{"artifact path", job("    steps: [{run: x}]\n    artifacts: [{name: x, paths: [../etc]}]\n"), CodePathInvalid},
		{"artifact absolute", job("    steps: [{run: x}]\n    artifacts: [{name: x, paths: [/etc]}]\n"), CodePathInvalid},
		{"cache home", job("    steps: [{run: x}]\n    cache: {key: k, paths: [~/.npm]}\n"), CodePathInvalid},
		{"artifact name", job("    steps: [{run: x}]\n    artifacts: [{name: 'a b', paths: [x]}]\n"), CodeArtifactNameInvalid},
		{"artifact duplicate in matrix", job("    strategy: {matrix: {os: [a, b]}}\n    steps: [{run: x}]\n    artifacts: [{name: out, paths: [x]}]\n"), CodeArtifactDuplicate},
		{"retention", job("    steps: [{run: x}]\n    artifacts: [{name: x, paths: [x], retention: 90d}]\n"), CodeRetentionInvalid},
		{"cache key", job("    steps: [{run: x}]\n    cache: {paths: [x]}\n"), CodeCacheKeyRequired},
		{"group", "version: 1\non: {push: {}}\nconcurrency: {cancel_in_progress: true}\njobs: {a: {image: " + img + ", steps: [{run: x}]}}\n", CodeGroupRequired},
		{"matrix empty axis", job("    strategy: {matrix: {os: []}}\n    steps: [{run: x}]\n"), CodeMatrixEmpty},
		{"matrix all excluded", job("    strategy: {matrix: {os: [a], exclude: [{os: a}]}}\n    steps: [{run: x}]\n"), CodeMatrixEmpty},
		{"matrix key", job("    strategy: {matrix: {'node-version': [1]}}\n    steps: [{run: x}]\n"), CodeMatrixKeyInvalid},
		{"matrix nested value", job("    strategy: {matrix: {os: [[a]]}}\n    steps: [{run: x}]\n"), CodeTypeMismatch},
		{"matrix include keys", job("    strategy: {matrix: {os: [a], include: [{arch: b}]}}\n    steps: [{run: x}]\n"), CodeMatrixEntryInvalid},
		{"matrix exclude keys", job("    strategy: {matrix: {os: [a, b], exclude: [{arch: b}]}}\n    steps: [{run: x}]\n"), CodeMatrixEntryInvalid},
		{"matrix duplicate", job("    strategy: {matrix: {os: [a, a]}}\n    steps: [{run: x}]\n"), CodeMatrixDuplicate},
		{"max parallel", job("    strategy: {max_parallel: 300}\n    steps: [{run: x}]\n"), CodeMaxParallelInvalid},
		{"environment name", job("    environment: Prod Env\n    steps: [{run: x}]\n"), CodeEnvironmentInvalid},
		{"type mismatch", job("    steps: {run: x}\n"), CodeTypeMismatch},
		{"bool as yes", "version: 1\non: {push: {}}\nconcurrency: {group: g, cancel_in_progress: yes}\njobs: {a: {image: " + img + ", steps: [{run: x}]}}\n", CodeTypeMismatch},
		{"step id duplicate", runJob("      - {run: y, id: s}\n      - {run: z, id: s}\n"), CodeStepIDDuplicate},
		{"plugin input name", runJob("      - uses: ghcr.io/x/y@sha256:" + digest + "\n        with: {Bad-Name: x}\n"), CodeInputInvalid},
		{"name expression", job("    name: '${{ git.ref }}'\n    steps: [{run: x}]\n"), CodeNameInvalid},
		{"empty file", "", CodeFileEmpty},
		{"comment only", "# nothing\n", CodeFileEmpty},
		{"two documents", runJob("") + "---\nversion: 1\n", CodeMultipleDocuments},
		{"syntax", "version: 1\non: [push\n", CodeYAMLSyntax},
		{"complex key", "? [a]\n: b\n", CodeKeyInvalid},
		{"root list", "- a\n", CodeTypeMismatch},
	}
}

func TestValidationErrors(t *testing.T) {
	cases := append(structureErrorCases(), policyErrorCases()...)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, issues := Check([]byte(tc.src))
			assert.Contains(t, issueCodes(issues, SeverityError), tc.code, "%v", issues)
			for _, i := range issues {
				assert.NotEmpty(t, i.Message)
				assert.Equal(t, strings.ToUpper(i.Code), i.Code)
			}
		})
	}
}

func TestWarnings(t *testing.T) {
	_, issues := Check([]byte(pipeline("  a:\n    image: alpine:3\n    environment: prod\n    env: {T: '${{ secrets.T }}'}\n    steps: [{run: make}]\n")))
	assert.Empty(t, issueCodes(issues, SeverityError), "%v", issues)
	assert.ElementsMatch(t, []string{CodeImageNotPinned, CodeEnvironmentsDeferred, CodeSecretsUnavailable}, issueCodes(issues, SeverityWarning))
}

func TestIssuesCarryPathAndPosition(t *testing.T) {
	_, issues := Check([]byte(runJob("      - run: echo ${{ git.message }}\n")))
	require.Len(t, issues, 1)
	assert.Equal(t, "jobs.a.steps[1].run", issues[0].Path)
	assert.Equal(t, Position{Line: 9, Column: 9}, issues[0].Position)
	assert.Contains(t, issues[0].String(), "9:9 error RUN_EXPRESSION_NOT_ALLOWED at jobs.a.steps[1].run")
}

func TestMessagesNeverEchoValues(t *testing.T) {
	secret := "ghp_" + strings.Repeat("S3cr3t", 6)
	sources := []string{
		job("    if: ${{ " + secret + " }}\n    steps: [{run: x}]\n"),
		job("    env: {A: 'x${{ " + secret + " }}'}\n    steps: [{run: x}]\n"),
		job("    image: " + secret + "!\n    steps: [{run: x}]\n"),
		job("    steps: [{run: 'echo ${{ " + secret + " }}'}]\n"),
		job("    " + secret + ": 1\n    steps: [{run: x}]\n"),
		"version: 1\non: [" + secret + "\n",
		job("    timeout: " + secret + "\n    steps: [{run: x}]\n"),
	}
	for i, src := range sources {
		_, issues := Check([]byte(src))
		require.NotEmpty(t, issues, "source %d", i)
		for _, is := range issues {
			text := is.Message + " " + is.Code
			assert.NotContains(t, text, secret, "source %d", i)
			e := &Error{Issues: []Issue{{Code: is.Code, Message: is.Message, Severity: is.Severity}}}
			assert.NotContains(t, e.Error(), secret)
		}
	}
}

func TestTooManyJobs(t *testing.T) {
	var b strings.Builder
	for i := range MaxJobs + 1 {
		fmt.Fprintf(&b, "  j%d:\n    image: %s\n    steps: [{run: x}]\n", i, img)
	}
	_, issues := Check([]byte(pipeline(b.String())))
	assert.Contains(t, issueCodes(issues, SeverityError), CodeTooManyJobs)
}

func TestTooManyRunJobs(t *testing.T) {
	var b strings.Builder
	values := make([]string, 0, 200)
	for i := range 200 {
		values = append(values, fmt.Sprint(i))
	}
	for i := range 2 {
		fmt.Fprintf(&b, "  j%d:\n    image: %s\n    strategy: {matrix: {n: [%s]}}\n    steps: [{run: x}]\n", i, img, strings.Join(values, ","))
	}
	_, issues := Check([]byte(pipeline(b.String())))
	assert.Contains(t, issueCodes(issues, SeverityError), CodeTooManyRunJobs)
}

func TestTooManySteps(t *testing.T) {
	var b strings.Builder
	for range 101 {
		b.WriteString("      - run: x\n")
	}
	_, issues := Check([]byte(job("    steps:\n" + b.String())))
	assert.Contains(t, issueCodes(issues, SeverityError), CodeTooManySteps)
}

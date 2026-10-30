package pipelinespec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func yamlBlocks(t *testing.T, rel string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", rel))
	require.NoError(t, err)
	var blocks []string
	var cur []string
	in := false
	for line := range strings.SplitSeq(string(raw), "\n") {
		switch {
		case !in && strings.TrimSpace(line) == "```yaml":
			in, cur = true, nil
		case in && strings.TrimSpace(line) == "```":
			in = false
			blocks = append(blocks, strings.Join(cur, "\n")+"\n")
		case in:
			cur = append(cur, line)
		}
	}
	return blocks
}

func TestEveryDocExamplePasses(t *testing.T) {
	for _, doc := range []string{"docs/pipeline-spec.md", "docs/en/pipeline-spec.md"} {
		blocks := yamlBlocks(t, doc)
		require.NotEmpty(t, blocks, doc)
		for i, src := range blocks {
			spec, issues := Check([]byte(src))
			require.NotNil(t, spec, "%s example %d: %v", doc, i+1, issues)
			assert.False(t, HasErrors(issues), "%s example %d: %v", doc, i+1, issues)
			plan, err := Compile(spec)
			require.NoError(t, err, "%s example %d", doc, i+1)
			assert.NotEmpty(t, plan.Jobs)
		}
	}
}

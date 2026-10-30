package pipelinespec

import (
	"strings"

	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/common/types"
)

type sizeEstimator struct{}

func maxTextFor(root string) uint64 {
	switch {
	case root == "git.message":
		return MaxMessageBytes
	case root == "env":
		return MaxContextValueBytes
	case root == "vars" || strings.HasPrefix(root, "inputs."):
		return MaxContextVarBytes
	case strings.HasPrefix(root, "matrix."):
		return 4 * MaxMatrixValue
	default:
		return MaxContextTextBytes
	}
}

func maxEntriesFor(root string, depth int) uint64 {
	switch {
	case root == "vars":
		return MaxContextVars
	case root == "env":
		return MaxContextEnv
	case strings.HasPrefix(root, "needs.") && depth == 1:
		return MaxMatrixJobs
	default:
		return MaxContextOutputs
	}
}

func (sizeEstimator) EstimateSize(n checker.AstNode) *checker.SizeEstimate {
	path := n.Path()
	if len(path) == 0 || n.Type() == nil {
		return nil
	}
	switch n.Type().Kind() {
	case types.StringKind:
		return &checker.SizeEstimate{Max: maxTextFor(path[0])}
	case types.MapKind, types.ListKind:
		return &checker.SizeEstimate{Max: maxEntriesFor(path[0], len(path))}
	default:
		return nil
	}
}

func (sizeEstimator) EstimateCallCost(function, _ string, _ *checker.AstNode, _ []checker.AstNode) *checker.CallEstimate {
	if function != hashFilesFunc {
		return nil
	}
	return &checker.CallEstimate{
		CostEstimate: checker.CostEstimate{Min: 1, Max: 1},
		ResultSize:   &checker.SizeEstimate{Min: hashHexLength, Max: hashHexLength},
	}
}

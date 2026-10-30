package pipelinespec

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"strconv"

	"go.yaml.in/yaml/v3"
)

const (
	tagStr       = "!!str"
	tagInt       = "!!int"
	tagFloat     = "!!float"
	tagBool      = "!!bool"
	tagNull      = "!!null"
	tagMap       = "!!map"
	tagSeq       = "!!seq"
	tagTimestamp = "!!timestamp"
	tagMerge     = "!!merge"
	mergeKey     = "<<"
)

var yamlErrorLine = regexp.MustCompile(`line (\d+)`)

func Parse(src []byte) (*Spec, []Issue) {
	l := &issueList{positions: map[string]Position{}}
	root, ok := parseDocument(src, l)
	if !ok {
		return nil, l.sorted()
	}
	t := &treeCheck{issues: l}
	t.walk(root, 1, "")
	if len(l.items) > 0 {
		return nil, l.sorted()
	}
	w := &schemaWalk{issues: l}
	w.walk(root, specType, "")
	if len(l.items) > 0 {
		return nil, l.sorted()
	}
	spec := &Spec{}
	if err := root.Decode(spec); err != nil {
		l.addAt(CodeTypeMismatch, "", Position{Line: root.Line, Column: root.Column})
		return nil, l.sorted()
	}
	spec.positions = l.positions
	return spec, nil
}

func parseDocument(src []byte, l *issueList) (*yaml.Node, bool) {
	if len(src) > MaxFileBytes {
		l.add(CodeFileTooLarge, "")
		return nil, false
	}
	dec := yaml.NewDecoder(bytes.NewReader(src))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			l.add(CodeFileEmpty, "")
		} else {
			l.addAt(CodeYAMLSyntax, "", errorPosition(err))
		}
		return nil, false
	}
	var extra yaml.Node
	switch err := dec.Decode(&extra); {
	case err == nil:
		l.addAt(CodeMultipleDocuments, "", Position{Line: extra.Line, Column: extra.Column})
		return nil, false
	case !errors.Is(err, io.EOF):
		l.addAt(CodeYAMLSyntax, "", errorPosition(err))
		return nil, false
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
		l.add(CodeFileEmpty, "")
		return nil, false
	}
	root := doc.Content[0]
	if root.Kind == yaml.ScalarNode && root.Tag == tagNull {
		l.add(CodeFileEmpty, "")
		return nil, false
	}
	return root, true
}

func errorPosition(err error) Position {
	m := yamlErrorLine.FindStringSubmatch(err.Error())
	if m == nil {
		return Position{}
	}
	line, convErr := strconv.Atoi(m[1])
	if convErr != nil {
		return Position{}
	}
	return Position{Line: line}
}

type treeCheck struct {
	issues  *issueList
	nodes   int
	tooDeep bool
	tooMany bool
}

func nodePos(n *yaml.Node) Position {
	return Position{Line: n.Line, Column: n.Column}
}

func (t *treeCheck) walk(n *yaml.Node, depth int, path string) {
	t.nodes++
	if t.nodes > MaxNodes {
		if !t.tooMany {
			t.tooMany = true
			t.issues.addAt(CodeTooManyNodes, path, nodePos(n))
		}
		return
	}
	if depth > MaxDepth {
		if !t.tooDeep {
			t.tooDeep = true
			t.issues.addAt(CodeDepthExceeded, path, nodePos(n))
		}
		return
	}
	t.node(n, path)
	switch n.Kind {
	case yaml.MappingNode:
		t.mapping(n, depth, path)
	case yaml.SequenceNode:
		for i, c := range n.Content {
			t.walk(c, depth+1, indexPath(path, i))
		}
	}
}

func (t *treeCheck) node(n *yaml.Node, path string) {
	switch {
	case n.Kind == yaml.AliasNode:
		t.issues.addAt(CodeAliasNotAllowed, path, nodePos(n))
	case n.Anchor != "":
		t.issues.addAt(CodeAnchorNotAllowed, path, nodePos(n))
	}
	if n.Kind == yaml.AliasNode {
		return
	}
	if !allowedTag(n) {
		t.issues.addAt(CodeTagNotAllowed, path, nodePos(n))
	}
	if n.Kind == yaml.ScalarNode && len(n.Value) > MaxScalarBytes {
		t.issues.addAt(CodeScalarTooLarge, path, nodePos(n))
	}
}

func allowedTag(n *yaml.Node) bool {
	switch n.Tag {
	case tagStr, tagInt, tagFloat, tagBool, tagNull, tagMap, tagSeq:
		return true
	case tagTimestamp:
		return n.Style&yaml.TaggedStyle == 0
	case tagMerge:
		return true
	default:
		return false
	}
}

func (t *treeCheck) mapping(n *yaml.Node, depth int, path string) {
	seen := map[string]bool{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, val := n.Content[i], n.Content[i+1]
		child := joinPath(path, key.Value)
		switch {
		case key.Kind == yaml.ScalarNode && (key.Tag == tagMerge || (key.Value == mergeKey && key.Style == 0)):
			t.issues.addAt(CodeMergeKeyNotAllowed, path, nodePos(key))
		case key.Kind != yaml.ScalarNode:
			t.issues.addAt(CodeKeyInvalid, path, nodePos(key))
		case seen[key.Value]:
			t.issues.addAt(CodeDuplicateKey, child, nodePos(key))
		}
		seen[key.Value] = true
		t.walk(key, depth+1, child)
		t.walk(val, depth+1, child)
	}
}

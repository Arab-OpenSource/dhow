package masking

import "sort"

const root = 0

type edge struct {
	b  byte
	to int32
}

type automaton struct {
	edges  [][]edge
	rootTo [256]int32
	fail   []int32
	out    []int32
	depth  []int32
	keep   []int32
	maxLen int
}

func newAutomaton(patterns [][]byte) *automaton {
	a := &automaton{edges: [][]edge{nil}, fail: []int32{0}, out: []int32{0}, depth: []int32{0}}
	for _, p := range patterns {
		a.insert(p)
	}
	a.link()
	return a
}

func (a *automaton) child(node int32, b byte) int32 {
	es := a.edges[node]
	i := sort.Search(len(es), func(i int) bool { return es[i].b >= b })
	if i < len(es) && es[i].b == b {
		return es[i].to
	}
	return -1
}

func (a *automaton) insert(p []byte) {
	if len(p) == 0 {
		return
	}
	node := int32(root)
	for _, b := range p {
		next := a.child(node, b)
		if next < 0 {
			next = int32(len(a.edges)) //nolint:gosec // Patterns come from at most 50 secrets, far below 2^31 nodes.
			a.edges = append(a.edges, nil)
			a.fail = append(a.fail, 0)
			a.out = append(a.out, 0)
			a.depth = append(a.depth, a.depth[node]+1)
			es := a.edges[node]
			i := sort.Search(len(es), func(i int) bool { return es[i].b >= b })
			es = append(es, edge{})
			copy(es[i+1:], es[i:])
			es[i] = edge{b: b, to: next}
			a.edges[node] = es
		}
		node = next
	}
	a.out[node] = int32(len(p)) //nolint:gosec // A pattern is a secret variant, far below 2^31 bytes.
	a.maxLen = max(a.maxLen, len(p))
}

func (a *automaton) link() {
	queue := make([]int32, 0, len(a.edges))
	for i := range a.rootTo {
		a.rootTo[i] = root
	}
	for _, e := range a.edges[root] {
		a.rootTo[e.b] = e.to
		a.fail[e.to] = root
		queue = append(queue, e.to)
	}
	a.keep = make([]int32, len(a.edges))
	order := []int32{root}
	for len(queue) > 0 {
		node := queue[0]
		order = append(order, node)
		queue = queue[1:]
		for _, e := range a.edges[node] {
			a.fail[e.to] = a.step(a.fail[node], e.b)
			a.out[e.to] = max(a.out[e.to], a.out[a.fail[e.to]])
			queue = append(queue, e.to)
		}
	}
	for _, node := range order {
		switch {
		case len(a.edges[node]) > 0:
			a.keep[node] = a.depth[node]
		case node != root:
			a.keep[node] = a.keep[a.fail[node]]
		}
	}
}

func (a *automaton) step(node int32, b byte) int32 {
	for node != root {
		if next := a.child(node, b); next >= 0 {
			return next
		}
		node = a.fail[node]
	}
	return a.rootTo[b]
}

func (a *automaton) empty() bool { return a.maxLen == 0 }

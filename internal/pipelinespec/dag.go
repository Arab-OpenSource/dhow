package pipelinespec

import "slices"

func topoOrder(needs map[string][]string) ([]string, []string) {
	indegree := make(map[string]int, len(needs))
	dependents := make(map[string][]string, len(needs))
	for id, deps := range needs {
		if _, ok := indegree[id]; !ok {
			indegree[id] = 0
		}
		for _, dep := range uniqueKnown(deps, needs) {
			indegree[id]++
			dependents[dep] = append(dependents[dep], id)
		}
	}
	ready := make([]string, 0, len(needs))
	for id, n := range indegree {
		if n == 0 {
			ready = append(ready, id)
		}
	}
	slices.Sort(ready)
	order := make([]string, 0, len(needs))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		order = append(order, id)
		for _, next := range dependents[id] {
			indegree[next]--
			if indegree[next] == 0 {
				i, _ := slices.BinarySearch(ready, next)
				ready = slices.Insert(ready, i, next)
			}
		}
	}
	var cyclic []string
	for id, n := range indegree {
		if n > 0 {
			cyclic = append(cyclic, id)
		}
	}
	slices.Sort(cyclic)
	return order, cyclic
}

func uniqueKnown(deps []string, known map[string][]string) []string {
	out := make([]string, 0, len(deps))
	for _, d := range deps {
		if _, ok := known[d]; ok && !slices.Contains(out, d) {
			out = append(out, d)
		}
	}
	return out
}

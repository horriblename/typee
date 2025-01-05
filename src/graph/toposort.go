package graph

import (
	"fmt"
	"maps"

	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/internal/ordered_set"
)

// Alias so I can easily move off any once I replace tarjan lib
type Node = string

type AdjacencyList = map[Node][]Node

// sorts a DAG
// err is [ErrCycle] if cycle is detected
// leaf nodes go first, root goes last
func Toposort(edges AdjacencyList) (_ []Node, err error) {
	sorted := []Node{}
	unvisited := orderedset.NewOrderedSet(fun.Collect(maps.Keys(edges))...)
	// false mark is a temporary mark, true is permanent
	mark := map[Node]bool{}

	// returns dependency cycle list, or nil
	var visit func(Node) []Node
	visit = func(node Node) []Node {
		unvisited.SwapDelete(node)
		perm, ok := mark[node]
		if ok {
			if perm {
				return nil
			} else {
				return []Node{node}
			}
		}

		mark[node] = false

		for _, m := range edges[node] {
			if cycle := visit(m); cycle != nil {
				return append(cycle, node)
			}
			mark[node] = true
		}

		sorted = append(sorted, node)
		return nil
	}

	for {
		if node, ok := unvisited.Pop(); ok {
			if cycle := visit(node); cycle != nil {
				return nil, fmt.Errorf("cycle detected: path unwind: %+s", cycle)
			}
		} else {
			break
		}
	}

	return sorted, nil
}

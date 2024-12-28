package graph

import (
	"fmt"
	"maps"

	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/internal/ordered_set"
)

// Alias so I can easily move off any once I replace tarjan lib
type Node = any

type AdjacencyList = map[Node][]Node

type ErrCycle struct {
	node Node
}

func (self ErrCycle) Error() string {
	return fmt.Sprintf("cycle detected, last node: %v", self.node)
}

// sorts a DAG
// err is [ErrCycle] if cycle is detected
// leaf nodes go first, root goes last
func Toposort(edges AdjacencyList) (_ []Node, err error) {
	defer func() {
		if e := recover(); e != nil {
			if ce, ok := e.(ErrCycle); ok {
				err = &ce
			} else {
				panic(e)
			}
		}
	}()

	sorted := []Node{}
	unvisited := orderedset.NewOrderedSet(fun.Collect(maps.Keys(edges))...)
	// false mark is a temporary mark, true is permanent
	mark := map[Node]bool{}

	var visit func(Node)
	visit = func(node Node) {
		unvisited.SwapDelete(node)
		perm, ok := mark[node]
		if ok {
			if perm {
				return
			} else {
				panic(ErrCycle{node})
			}
		}

		mark[node] = false

		for _, m := range edges[node] {
			visit(m)
			mark[node] = true
		}

		sorted = append(sorted, node)
	}

	for {
		if node, ok := unvisited.Pop(); ok {
			visit(node)
		} else {
			break
		}
	}

	return sorted, nil
}

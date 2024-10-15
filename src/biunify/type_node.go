package biunify

import (
	"github.com/horriblename/typee/src/assert"
)

//go-sumtype: TypeNode

type TypeNode interface{ typeNode() }

type Var struct{}
type VNode struct{ Head VTypeHead }
type UNode struct{ Head UTypeHead }

func (Var) typeNode()   {}
func (VNode) typeNode() {}
func (UNode) typeNode() {}

type unit struct{}
type Reachability struct {
	// all the nodes that have edges to a given node
	// e.g. upSets[id] is the set of all nodes that can reach id
	upSets []OrderedSet[ID]

	// downSets[id] is the set of nodes we can reach from id
	downSets []OrderedSet[ID]
}

type Edge struct {
	From ID
	To   ID
}

func (self *Reachability) addNode() ID {
	id := len(self.upSets)
	self.upSets = append(self.upSets, NewOrderedSet[ID]())
	self.downSets = append(self.downSets, NewOrderedSet[ID]())
	return id
}

func (self *Reachability) addEdge(lhs ID, rhs ID, out *[]Edge) {
	work := []Edge{{lhs, rhs}}

	for len(work) > 0 {
		edge, ok := popSlice(&work).Unwrap()
		assert.True(ok, "pop returned empty despite len check")

		if self.downSets[edge.From].insert(edge.To) {
			continue
		}
		self.upSets[edge.To].insert(edge.From)
		// inform caller that a new edge was added
		*out = append(*out, edge)

		for _, lhs2 := range self.upSets[edge.From].slice() {
			work = append(work, Edge{lhs2, edge.To})
		}
		for _, rhs2 := range self.downSets[edge.To].slice() {
			work = append(work, Edge{edge.From, rhs2})
		}
	}
}

type OrderedSet[T comparable] struct {
	list    []T
	mapping map[T]struct{}
}

func NewOrderedSet[T comparable]() OrderedSet[T] {
	return OrderedSet[T]{
		list:    []T{},
		mapping: map[T]struct{}{},
	}
}

func (set OrderedSet[T]) insert(x T) (existed bool) {
	_, existed = set.mapping[x]
	if !existed {
		set.mapping[x] = struct{}{}
		set.list = append(set.list, x)
	}

	return existed
}

func (set OrderedSet[T]) has(x T) bool {
	_, found := set.mapping[x]
	return found
}

func (set OrderedSet[T]) slice() []T {
	return set.list
}

func (set OrderedSet[T]) len() int {
	return len(set.list)
}

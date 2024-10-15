package biunify

import (
	"slices"
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
	var set1, set2 OrderedSet[ID]
	set1.insert(id)
	set2.insert(id)

	self.upSets = append(self.upSets, set1)
	self.downSets = append(self.downSets, set2)
	return id
}

func (self *Reachability) addEdge(lhs ID, rhs ID, out *[]Edge) {
	if self.downSets[lhs].has(rhs) {
		return
	}

	// Get all ancestores of lhs, including lhs itself
	lhsSet := make([]ID, 0, self.upSets[lhs].len())
	for _, id := range self.upSets[lhs].slice() {
		lhsSet = append(lhsSet, id)
	}
	slices.Sort(lhsSet)

	// Get all descendants of rhs, including rhs itself
	rhsSet := make([]ID, 0, self.downSets[rhs].len())
	for _, id := range self.downSets[rhs].slice() {
		rhsSet = append(rhsSet, id)
	}
	slices.Sort(rhsSet)

	for _, lhs2 := range lhsSet {
		for _, rhs2 := range rhsSet {
			if !self.downSets[lhs2].insert(rhs2) {
				self.upSets[rhs2].insert(lhs2)
				*out = append(*out, Edge{lhs2, rhs2})
			}
		}
	}
}

type OrderedSet[T comparable] struct {
	list    []T
	mapping map[T]struct{}
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

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
	upSets []set[ID]

	// downSets[id] is the set of nodes we can reach from id
	downSets []set[ID]
}

type Edge struct {
	From ID
	To   ID
}

func (self *Reachability) addNode() ID {
	id := len(self.upSets)
	var set1, set2 set[ID]
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
	lhsSet := make([]ID, 0, len(self.upSets[lhs]))
	for id, _ := range self.upSets[lhs] {
		lhsSet = append(lhsSet, id)
	}
	slices.Sort(lhsSet)

	// Get all descendants of rhs, including rhs itself
	rhsSet := make([]ID, 0, len(self.downSets[rhs]))
	for id, _ := range self.downSets[rhs] {
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

type set[T comparable] map[T]struct{}

func (set set[T]) insert(x T) (existed bool) {
	_, existed = set[x]
	if !existed {
		set[x] = struct{}{}
	}

	return existed
}

func (set set[T]) remove(x T) (existed bool) {
	_, existed = set[x]
	if existed {
		delete(set, x)
	}

	return existed
}

func (set set[T]) has(x T) bool {
	_, found := set[x]
	return found
}

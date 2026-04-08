package scope

import (
	"maps"

	"github.com/horriblename/typee/src/internal/ordered"
	"github.com/horriblename/typee/src/opt"
)

type scopeChange[T any] struct {
	Key string
	Old opt.Option[T]
}

type ScopedMap[T any] struct {
	table      map[string]T
	changes    []scopeChange[T]
	savePoints []int
}

func NewScopedMap[T any]() ScopedMap[T] {
	return ScopedMap[T]{table: map[string]T{}, changes: []scopeChange[T]{}}
}

// A ScopedMap with a starting globals table that cannot be removed via [ScopedMap.PopScope]
// m will be cloned
func ScopedMapWithGlobals[T any](m map[string]T) ScopedMap[T] {
	return ScopedMap[T]{
		table:      maps.Clone(m),
		changes:    []scopeChange[T]{},
		savePoints: []int{},
	}
}

func (self *ScopedMap[T]) Get(k string) opt.Option[T] {
	if v, found := self.table[k]; found {
		return opt.Some(v)
	}

	return opt.None[T]()
}

func (self *ScopedMap[T]) Insert(k string, v T) {
	change := scopeChange[T]{Key: k}
	if old, found := self.table[k]; found {
		change.Old = opt.Some(old)
	}
	self.changes = append(self.changes, change)
	self.table[k] = v
}

func (self *ScopedMap[T]) NewScope() {
	self.savePoints = append(self.savePoints, len(self.changes))
}

func (self *ScopedMap[T]) PopScope() ordered.Map[string, T] {
	savePoint := self.savePoints[len(self.savePoints)-1]
	popped := ordered.NewMap[string, T]()
	for i := len(self.changes) - 1; i >= savePoint; i-- {
		change := self.changes[i]
		if old, found := change.Old.Unwrap(); found {
			self.table[change.Key] = old
		} else {
			popped.Insert(change.Key, self.table[change.Key])
			delete(self.table, change.Key)
		}
	}
	self.changes = self.changes[:savePoint]
	self.savePoints = self.savePoints[:len(self.savePoints)-1]
	// TODO: should shrink savePoints
	return popped
}

func (self *ScopedMap[T]) ScopeLevel() int {
	return len(self.savePoints)
}

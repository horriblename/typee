package biunify

import (
	"github.com/horriblename/typee/src/opt"
)

type bindChange struct {
	Key   string
	Value opt.Option[Value]
}

type Bindings struct {
	m          map[string]Value
	changes    []bindChange
	savePoints []int
}

func NewBindings() Bindings {
	return Bindings{m: map[string]Value{}, changes: []bindChange{}}
}

func (self *Bindings) get(k string) opt.Option[Value] {
	if v, found := self.m[k]; found {
		return opt.Some(v)
	}

	return opt.None[Value]()
}

func (self *Bindings) insert(k string, v Value) {
	change := bindChange{Key: k}
	if old, found := self.m[k]; found {
		change.Value = opt.Some(old)
	}
	self.changes = append(self.changes, change)
	self.m[k] = v
}

func (self *Bindings) NewScope() {
	self.savePoints = append(self.savePoints, len(self.changes))
}

func (self *Bindings) PopScope() {
	savePoint := self.savePoints[len(self.savePoints)-1]
	for i := len(self.changes) - 1; i >= savePoint; i-- {
		change := self.changes[i]
		if old, found := change.Value.Unwrap(); found {
			self.m[change.Key] = old
		} else {
			delete(self.m, change.Key)
		}
	}
	self.savePoints = self.savePoints[:len(self.savePoints)-1]
	// TODO: should shrink savePoints
}

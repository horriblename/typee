package biunify

import "github.com/horriblename/typee/src/opt"

type Bindings struct {
	m []map[string]Value
}

func NewBindings() Bindings {
	return Bindings{m: []map[string]Value{{}}}
}

func (self *Bindings) get(k string) opt.Option[Value] {
	for i := len(self.m) - 1; i > 0; i-- {
		if v, found := self.m[i][k]; found {
			return opt.Some(v)
		}
	}

	return opt.None[Value]()
}

func (self *Bindings) insert(k string, v Value) {
	if len(self.m) == 0 {
		panic("assertion failed: tried to insert into Bindings with no scopes")
	}

	self.m[len(self.m)-1][k] = v
}

func (self *Bindings) NewScope() {
	self.m = append(self.m, map[string]Value{})
}

func (self *Bindings) PopScope() {
	self.m[len(self.m)-1] = nil
	self.m = self.m[:len(self.m)-1]
}

package biunify

import "github.com/horriblename/typee/src/assert"

type TypeCheckerCore struct {
	reachability Reachability
	types        []TypeNode
}

func (self *TypeCheckerCore) newVal(valType VTypeHead) Value {
	id := self.reachability.addNode()
	assert.Eq(id, len(self.types))
	self.types = append(self.types, VNode{valType})
	return Value{id}
}

func (self *TypeCheckerCore) newUse(constraint UTypeHead) Use {
	id := self.reachability.addNode()
	assert.Eq(id, len(self.types))
	self.types = append(self.types, UNode{constraint})
	return Use{ID: id}
}

func (self *TypeCheckerCore) var_() TypePair {
	id := self.reachability.addNode()
	assert.Eq(id, len(self.types))
	self.types = append(self.types, Var{})
	return TypePair{Value{id}, Use{ID: id}}
}

func (self *TypeCheckerCore) bool() Value {
	return self.newVal(VBool{})
}

func (self *TypeCheckerCore) boolUse() Use {
	return self.newUse(UBool{})
}

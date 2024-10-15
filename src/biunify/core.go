package biunify

import (
	"fmt"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/opt"
)

type TypeCheckerCore struct {
	reachability Reachability
	types        []TypeNode
}

func (self *TypeCheckerCore) NewVal(valType VTypeHead) Value {
	id := self.reachability.addNode()
	assert.Eq(id, len(self.types))
	self.types = append(self.types, VNode{valType})
	return Value{id}
}

func (self *TypeCheckerCore) NewUse(constraint UTypeHead) Use {
	id := self.reachability.addNode()
	assert.Eq(id, len(self.types))
	self.types = append(self.types, UNode{constraint})
	return Use{ID: id}
}

func (self *TypeCheckerCore) Var() (Value, Use) {
	id := self.reachability.addNode()
	assert.Eq(id, len(self.types))
	self.types = append(self.types, Var{})
	return Value{id}, Use{ID: id}
}

func (self *TypeCheckerCore) Bool() Value {
	return self.NewVal(VBool{})
}

func (self *TypeCheckerCore) BoolUse() Use {
	return self.NewUse(UBool{})
}

func (self *TypeCheckerCore) Func(args []Use, ret Value) Value {
	return self.NewVal(VFunc{
		Arg: args,
		Ret: ret,
	})
}

func (self *TypeCheckerCore) FuncUse(arg []Value, ret Use) Use {
	return self.NewUse(UFunc{
		Arg: arg,
		Ret: ret,
	})
}

func (self *TypeCheckerCore) Obj(fields []NamedValue) Value {
	fieldsMap := map[string]Value{}
	for _, field := range fields {
		fieldsMap[field.Name] = field.Value
	}

	return self.NewVal(VObj{
		Fields: fieldsMap,
	})
}

func (self *TypeCheckerCore) ObjUse(field NamedUse) Use {
	return self.NewUse(UObj{Field: field})
}

func (self *TypeCheckerCore) Tagged(val NamedValue) Value {
	return self.NewVal(VTagged(val))
}

func (self *TypeCheckerCore) TaggedUse(variants []NamedUse) Use {
	variantsMap := map[string]Use{}
	for _, variant := range variants {
		variantsMap[variant.Name] = variant.Use
	}

	return self.NewUse(UTagged{variantsMap})
}

func (self *TypeCheckerCore) Flow(lhs Value, rhs Use) error {
	fmt.Printf("#%d%#v <= #%d%#v\n", lhs.ID, self.types[lhs.ID], rhs.ID, self.types[rhs.ID])
	var err error
	pendingEdges := []TypePair{{lhs, rhs}}
	typePairsToCheck := []Edge{}
	for len(pendingEdges) > 0 {
		edge, ok := popSlice(&pendingEdges).Unwrap()
		assert.True(ok, "pop non-empty slice got empty result")

		self.reachability.addEdge(edge.Value.ID, edge.Use.ID, &typePairsToCheck)

		for len(typePairsToCheck) > 0 {
			pair, ok := popSlice(&typePairsToCheck).Unwrap()
			assert.True(ok, "pop non-empty slice got empty result")

			lhsHead, ok := self.types[pair.From].(VNode)
			if !ok {
				continue
			}

			rhsHead, ok := self.types[pair.To].(UNode)
			if !ok {
				continue
			}

			pendingEdges, err = CheckHeads(lhsHead.Head, rhsHead.Head, pendingEdges)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (self *TypeCheckerCore) Head(id ID) TypeNode {
	return self.types[id]
}

func popSlice[T any](s *[]T) opt.Option[T] {
	var zero T
	if len(*s) > 0 {
		ret := opt.Some((*s)[len(*s)-1])
		(*s)[len(*s)-1] = zero
		*s = (*s)[:len(*s)-1]
		return ret
	}

	return opt.None[T]()
}

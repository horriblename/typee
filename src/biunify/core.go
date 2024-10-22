package biunify

import (
	"errors"
	"fmt"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/biunify/internal/reachable"
	"github.com/horriblename/typee/src/opt"
)

var ErrIncompatibleKind = errors.New("incompatible kinds")

type TypeCheckerCore struct {
	reachability reachable.Reachability
	types        []TypeNode
}

func (self *TypeCheckerCore) newVal(valType VTypeHead) Value {
	id := self.reachability.AddNode()
	assert.Eq(id, len(self.types))
	self.types = append(self.types, VNode{valType})
	return Value{id}
}

func (self *TypeCheckerCore) newUse(constraint UTypeHead) Use {
	id := self.reachability.AddNode()
	assert.Eq(id, len(self.types))
	self.types = append(self.types, UNode{constraint})
	return Use{ID: id}
}

func (self *TypeCheckerCore) Var() (Value, Use) {
	id := self.reachability.AddNode()
	assert.Eq(id, len(self.types))
	self.types = append(self.types, Var{})
	return Value{id}, Use{ID: id}
}

func (self *TypeCheckerCore) Bool() Value {
	return self.newVal(VBool{})
}

func (self *TypeCheckerCore) BoolUse() Use {
	return self.newUse(UBool{})
}

func (self *TypeCheckerCore) Int() Value {
	return self.newVal(VInt{})
}

func (self *TypeCheckerCore) IntUse() Use {
	return self.newUse(UInt{})
}

func (self *TypeCheckerCore) Str() Value {
	return self.newVal(VStr{})
}

func (self *TypeCheckerCore) StrUse() Use {
	return self.newUse(UStr{})
}

func (self *TypeCheckerCore) Func(args []Use, ret Value) Value {
	return self.newVal(VFunc{
		Arg: args,
		Ret: ret,
	})
}

func (self *TypeCheckerCore) FuncUse(arg []Value, ret Use) Use {
	return self.newUse(UFunc{
		Arg: arg,
		Ret: ret,
	})
}

func (self *TypeCheckerCore) Obj(fields []NamedValue) Value {
	fieldsMap := map[string]Value{}
	for _, field := range fields {
		fieldsMap[field.Name] = field.Value
	}

	return self.newVal(VObj{
		Fields: fieldsMap,
	})
}

func (self *TypeCheckerCore) ObjUse(field NamedUse) Use {
	return self.newUse(UObj{Field: field})
}

func (self *TypeCheckerCore) Tagged(val NamedValue) Value {
	return self.newVal(VTagged(val))
}

func (self *TypeCheckerCore) TaggedUse(variants []NamedUse) Use {
	variantsMap := map[string]Use{}
	for _, variant := range variants {
		variantsMap[variant.Name] = variant.Use
	}

	return self.newUse(UTagged{variantsMap})
}

func (self *TypeCheckerCore) Flow(lhs Value, rhs Use) error {
	fmt.Printf("[flow] #%d%#v <= #%d%#v\n", lhs.ID, self.types[lhs.ID], rhs.ID, self.types[rhs.ID])
	var err error
	pendingEdges := []TypePair{{lhs, rhs}}
	typePairsToCheck := []reachable.Edge{}
	for len(pendingEdges) > 0 {
		fmt.Printf("[flow] left to add: %v \n", pendingEdges)

		edge, ok := popSlice(&pendingEdges).Unwrap()
		assert.True(ok, "pop non-empty slice got empty result")

		self.reachability.AddEdge(edge.Value.ID, edge.Use.ID, &typePairsToCheck)

		for len(typePairsToCheck) > 0 {
			fmt.Printf("[flow] left to check: %v \n", typePairsToCheck)
			pair, ok := popSlice(&typePairsToCheck).Unwrap()
			assert.True(ok, "pop non-empty slice got empty result")

			switch lhsHead := self.types[pair.From].(type) {
			case Var:
				if err := self.budgetUnify(pair.From, pair.To); err != nil {
					return err
				}

			case VNode:
				switch rhsHead := self.types[pair.To].(type) {
				case UNode:
					pendingEdges, err = CheckHeads(lhsHead.Head, rhsHead.Head, pendingEdges)
					if err != nil {
						return err
					}

				case Var:
					if err := self.budgetUnify(pair.From, pair.To); err != nil {
						return err
					}
				}
			}
		}
	}

	assert.Eq(len(pendingEdges), 0)
	assert.Eq(len(typePairsToCheck), 0)
	return nil
}

func (self *TypeCheckerCore) Head(id ID) TypeNode {
	return self.types[id]
}

func (self *TypeCheckerCore) budgetUnify(left ID, right ID) error {
	lhs := self.types[left]
	rhs := self.types[right]
	fmt.Printf("[budgetUnify] unifying %#v and %#v\n", lhs, rhs)
	// TODO: check that children of composite types are compatible,
	// e.g. {foo: Str} cannot unify with {foo: Int}
	lhsVar, ok := lhs.(Var)
	if !ok {
		if _, ok := rhs.(Var); !ok {
			return nil
		}
		return self.budgetUnify(right, left)
	}
	switch rhsVariant := rhs.(type) {
	case UNode:
		rhsHead := rhsVariant.Head
		if lhsVar.Kind == nil {
			self.types[left] = Var{Kind: rhsHead}
			return nil
		} else if matchUHeads(lhsVar.Kind, rhsHead) {
			return nil
		} else {
			return fmt.Errorf("%w: could not unify %T and %T", ErrIncompatibleKind, lhsVar.Kind, rhsHead)
		}
	case VNode:
		rhsHead := vHeadToUHeadTypeOnly(rhsVariant.Head)
		if lhsVar.Kind == nil {
			self.types[left] = Var{Kind: rhsHead}
			return nil
		} else if matchUHeads(lhsVar.Kind, rhsHead) {
			return nil
		} else {
			return fmt.Errorf("%w: could not unify %T and %T", ErrIncompatibleKind, lhsVar.Kind, rhsHead)
		}

	case Var:
		if lhsVar.Kind == nil {
			self.types[left] = Var{Kind: rhsVariant.Kind}
			return nil
		} else if rhsVariant.Kind == nil {
			self.types[right] = &Var{Kind: lhsVar.Kind}
			return nil
		}

		if matchUHeads(lhsVar.Kind, rhsVariant.Kind) {
			return nil
		} else {
			return fmt.Errorf("%w: could not unify %T and %T", ErrIncompatibleKind, lhsVar.Kind, rhsVariant.Kind)
		}
	default:
		panic("unexpected biunify.TypeNode")
	}
}

// does not copy inner fields of VTypeHead
func vHeadToUHeadTypeOnly(v VTypeHead) UTypeHead {
	switch v.(type) {
	case VBool:
		return UBool{}
	case VFunc:
		return UFunc{}
	case VObj:
		return UObj{}
	case VTagged:
		return UTagged{}
	}
	panic("unreachable")
}

func matchUHeads(lhs UTypeHead, rhs UTypeHead) bool {
	return is[UBool](lhs) && is[UBool](rhs) ||
		is[UInt](lhs) && is[UInt](rhs) ||
		is[UStr](lhs) && is[UStr](rhs) ||
		is[UTagged](lhs) && is[UTagged](rhs) ||
		is[UObj](lhs) && is[UObj](rhs)
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

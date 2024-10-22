package biunify

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/biunify_simpler/internal/ordered_set"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/opt"
)

var ErrInvalidCyclicConstraint = errors.New("invalid cyclic constraint")

// TypeScheme is a type that potentially contains universally quantified type variables.
// can be instantiated to a given level
type TypeScheme interface {
	instantiate() SimpleType
}

// PolymorphicType is a type with universally quantified type variables
type PolymorphicType struct {
	body SimpleType
}

func (self PolymorphicType) instantiate() SimpleType { panic("unimpl") }

// SimpleType is a type without universally quantified type variables
type SimpleType interface {
	TypeScheme
	children() []SimpleType
}

type Variable struct {
	lowerBound     ConcreteType
	upperBound     ConcreteType
	representative *Variable
}

func (self Variable) instantiate() SimpleType { return self }
func (self Variable) children() []SimpleType {
	return []SimpleType{self.lowerBound, self.upperBound}
}

func (self Variable) LowerBound() ConcreteType { return self.lowerBound }
func (self Variable) UpperBound() ConcreteType { return self.upperBound }
func (self *Variable) newUpperBound(ub ConcreteType) (SimpleType, error) {
	if err := self.occursCheck(ub, true); err != nil {
		return nil, err
	}

	// rep := self.representative
	panic("unimpl")
}
func (self *Variable) newLowerBound(lb ConcreteType) error {
	var err error
	self.occursCheck(lb, false)
	rep := self.Representative()

	rep.lowerBound, err = lubConcrete(rep.lowerBound, lb)
	if err != nil {
		return err
	}

	return constrain(lb, rep.upperBound)
}
func (self *Variable) Representative() *Variable {
	if self.representative != nil {
		return self.representative
	} else {
		return self
	}
}

func (self Variable) occursCheck(ty ConcreteType, dir bool) error {
	if getVars(ty).Has(*self.Representative()) {
		relation := ":>"
		if dir {
			relation = "<:"
		}
		return fmt.Errorf("%w: %v %s %v", ErrInvalidCyclicConstraint, self, relation, ty)
	}
	return nil
}

func glbConcrete(lhs ConcreteType, rhs ConcreteType) (ConcreteType, error) {
	type C = ConcreteType
	if _, rhs, ok := matchPair[Top, C](lhs, rhs); ok {
		return rhs, nil
	} else if lhs, _, ok := matchPair[C, Top](lhs, rhs); ok {
		return lhs, nil
	} else if _, rhs, ok := matchPair[Bot, C](lhs, rhs); ok {
		return rhs, nil
	} else if lhs, _, ok := matchPair[C, Bot](lhs, rhs); ok {
		return lhs, nil
	} else if lhs, rhs, ok := matchPair[Func, Func](lhs, rhs); ok {
		args := make([]SimpleType, 0, len(lhs.Args))
		argPairs := fun.ZipIter(slices.Values(lhs.Args), slices.Values(rhs.Args))
		for pair := range argPairs {
			arg, err := lub(pair.One, pair.Two)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}

		ret, err := glb(lhs.Ret, rhs.Ret)
		if err != nil {
			return nil, err
		}

		return Func{args, ret}, nil
	} else if lhs, rhs, ok := matchPair[Record, Record](lhs, rhs); ok {
		var err error
		lhsMap := namedTypesToMap(lhs.Fields)
		rhsMap := namedTypesToMap(rhs.Fields)

		mergedMap := maps.Clone(lhsMap)
		for rhsKey, rhsVal := range rhsMap {
			if lhsVal, ok := mergedMap[rhsKey]; ok {
				mergedMap[rhsKey], err = glb(lhsVal, rhsVal)
				if err != nil {
					return nil, err
				}

			}
		}
		return Record{mapToNamedTypes(mergedMap)}, nil
	} else if _, _, ok := matchPair[Bool, Bool](lhs, rhs); ok {
		return Bool{}, nil
	} else {
		return Bot{}, nil
	}
}

func lubConcrete(lhs ConcreteType, rhs ConcreteType) (ConcreteType, error) {
	type C = ConcreteType
	if lhs, rhs, ok := matchPair[Bot, C](lhs, rhs); ok {
		return rhs, nil
	} else if lhs, rhs, ok := matchPair[C, Bot](lhs, rhs); ok {
		return lhs, nil
	} else if _, _, ok := matchPair[Top, C](lhs, rhs); ok {
		return Top{}, nil
	} else if _, _, ok := matchPair[C, Top](lhs, rhs); ok {
		return Top{}, nil
	} else if lhs, rhs, ok := matchPair[Func, Func](lhs, rhs); ok {
		assert.Eq(len(lhs.Args), len(rhs.Args), "different arg count")

		args := make([]SimpleType, len(lhs.Args))
		for i, lhsArg := range lhs.Args {
			ty, err := glb(lhsArg, rhs.Args[i])
			if err != nil {
				return nil, err
			}
			args = append(args, ty)
		}

		ret, err := lub(lhs.Ret, rhs.Ret)
		if err != nil {
			return nil, err
		}

		return Func{args, ret}, nil
	} else if lhs, rhs, ok := matchPair[Record, Record](lhs, rhs); ok {
		// the "intersection" of both records
		rhsMap := namedTypesToMap(rhs.Fields)

		merged := []NamedType{}
		for _, lhsField := range lhs.Fields {
			if rhsField, ok := rhsMap[lhsField.Name]; ok {
				// TODO: reject "union" types like int|string
				ty, err := lub(lhsField.Type, rhsField)
				if err != nil {
					return nil, err
				}

				merged = append(merged, NamedType{
					Name: lhsField.Name,
					Type: ty,
				})
			}
		}
		return Record{Fields: merged}, nil
	} else if _, _, ok := matchPair[Bool, Bool](lhs, rhs); ok {
		return Bool{}, nil
	} else {
		return Top{}, nil
	}
}

func glb(lhs SimpleType, rhs SimpleType) (SimpleType, error) {
	if lhs, rhs, ok := matchPair[ConcreteType, ConcreteType](lhs, rhs); ok {
		return glbConcrete(lhs, rhs)
	} else if lhs, rhs, ok := matchPair[Variable, Variable](lhs, rhs); ok {
		return unify(lhs, rhs)
	} else if lhs, rhs, ok := matchPair[ConcreteType, Variable](lhs, rhs); ok {
		return rhs.newUpperBound(lhs)
	}
	panic("TODO")
}
func lub(SimpleType, SimpleType) (SimpleType, error) {
	panic("unimpl")
}

func matchPair[L, R any](lhs interface{}, rhs interface{}) (_ L, _ R, ok bool) {
	l, ok0 := lhs.(L)
	r, ok1 := rhs.(R)

	return l, r, ok0 && ok1
}

// SimpleType that is not a [Variable]
type ConcreteType interface {
	SimpleType
	concrete()
}

type Top struct{}
type Bot struct{}
type Func struct {
	Args []SimpleType
	Ret  SimpleType
}
type Record struct{ Fields []NamedType }
type Bool struct{}

func (self Top) instantiate() SimpleType    { return self }
func (self Bot) instantiate() SimpleType    { return self }
func (self Func) instantiate() SimpleType   { return self }
func (self Record) instantiate() SimpleType { return self }
func (self Bool) instantiate() SimpleType   { return self }

func (self Top) children() []SimpleType { return []SimpleType{} }
func (self Bot) children() []SimpleType { return []SimpleType{} }
func (self Func) children() []SimpleType {
	return append(append([]SimpleType{}, self.Args...), self.Ret)
}
func (self Record) children() []SimpleType {
	return fun.Map(self.Fields, func(field NamedType) SimpleType { return field.Type })
}
func (self Bool) children() []SimpleType { return []SimpleType{} }

func (self Top) concrete()    {}
func (self Bot) concrete()    {}
func (self Func) concrete()   {}
func (self Record) concrete() {}
func (self Bool) concrete()   {}

func getVars(ty SimpleType) *orderedset.OrderedSet[Variable] {
	result := orderedset.NewOrderedSet[Variable]()
	work := []SimpleType{ty}

	for len(work) > 0 {
		ty, ok := popSlice(&work).Unwrap()
		assert.True(ok, "pop returned empty despite len check")

		if v, ok := ty.(Variable); ok {
			if result.Has(v) {
				continue
			}
			result.Insert(v)
			work = append(work, v.children()...)
		} else {
			work = append(work, v.children()...)
		}
	}

	return result
}

type SimpleTypeImpl struct{}

// helpers

type NamedType struct {
	Name string
	Type SimpleType
}

type unit struct{}

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

func namedTypesToMap(fields []NamedType) map[string]SimpleType {
	m := map[string]SimpleType{}
	for _, field := range fields {
		m[field.Name] = field.Type
	}
	return m
}

func mapToNamedTypes(fields map[string]SimpleType) []NamedType {
	s := make([]NamedType, 0, len(fields))
	for k, v := range fields {
		s = append(s, NamedType{Name: k, Type: v})
	}
	return s
}

func mergeMapWith[K comparable, V any](lhs map[K]V, rhs map[K]V, merge func(l V, r V) V) map[K]V {
	merged := maps.Clone(lhs)
	for rhsKey, rhsVal := range rhs {
		if lhsVal, ok := merged[rhsKey]; ok {
			merged[rhsKey] = merge(lhsVal, rhsVal)
		}
	}
	return merged
}

func err2Func2ToResultFunc[I1, I2, O any](f func(I1, I2) (O, error)) func(I1, I2) fun.Result[O] {
	return func(i1 I1, i2 I2) fun.Result[O] {
		return fun.ResultFrom(f(i1, i2))
	}
}

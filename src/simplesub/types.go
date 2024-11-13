package simplesub

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/opt"
	"github.com/horriblename/typee/src/simplesub/internal/ordered_set"
	"github.com/horriblename/typee/src/types"
)

//go-sumtype:decl TypeScheme SimpleType ConcreteType

var ErrInvalidCyclicConstraint = errors.New("invalid cyclic constraint")
var ErrIncompatibleTypes = errors.New("incompatible types")

// TypeScheme is a type that potentially contains universally quantified type variables.
// can be instantiated to a given level
type TypeScheme interface {
	instantiate() SimpleType
}

// PolymorphicType is a type with universally quantified type variables
type PolymorphicType struct {
	Body SimpleType
}

func (self PolymorphicType) instantiate() SimpleType {
	return freshenType(self.Body)
}

// SimpleType is a type without universally quantified type variables
type SimpleType interface {
	TypeScheme
	children() []SimpleType
	String() string
}

type Variable struct {
	uid            uint
	lowerBound     ConcreteType
	upperBound     ConcreteType
	representative *Variable // nilable
}

func (self *Variable) instantiate() SimpleType { return self }
func (self *Variable) children() []SimpleType {
	return []SimpleType{self.lowerBound, self.upperBound}
}
func (self *Variable) Uid() uint {
	return self.Representative().uid
}
func (self *Variable) asTypeVar() types.Type {
	return &types.Generic{ID: types.TypeID(self.uid)}
}

func (self *Variable) String() string {
	if self.representative != nil {
		return fmt.Sprintf("t%d=t%d[%v, %v]", self.uid, self.representative.uid, self.lowerBound, self.upperBound)
	}
	return fmt.Sprintf("t%d[%v, %v]", self.uid, self.lowerBound, self.upperBound)
}
func (self *Variable) LowerBound() ConcreteType { return self.lowerBound }
func (self *Variable) UpperBound() ConcreteType { return self.upperBound }
func (self *Variable) newUpperBound(ub ConcreteType) error {
	if err := self.occursCheck(ub, true); err != nil {
		return err
	}

	rep := self.Representative()
	newUb, err := glbConcrete(rep.upperBound, ub)
	if err != nil {
		return err
	}
	rep.upperBound = newUb

	return constrain(rep.lowerBound, ub)
}
func (self *Variable) newLowerBound(lb ConcreteType) error {
	var err error
	if err := self.occursCheck(lb, false); err != nil {
		return err
	}

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

func (self *Variable) occursCheck(ty ConcreteType, dir bool) error {
	if getVars(ty).Has(self.Representative()) {
		relation := ":>"
		if dir {
			relation = "<:"
		}
		return fmt.Errorf("%w: %v %s %v", ErrInvalidCyclicConstraint, self, relation, ty)
	}
	return nil
}

func glbConcrete(lhs0 ConcreteType, rhs0 ConcreteType) (ConcreteType, error) {
	type C = ConcreteType
	if _, rhs, ok := matchPair[Top, C](lhs0, rhs0); ok {
		return rhs, nil
	} else if lhs, _, ok := matchPair[C, Top](lhs0, rhs0); ok {
		return lhs, nil
	} else if _, rhs, ok := matchPair[Bot, C](lhs0, rhs0); ok {
		return rhs, nil
	} else if lhs, _, ok := matchPair[C, Bot](lhs0, rhs0); ok {
		return lhs, nil
	} else if lhs, rhs, ok := matchPair[Func, Func](lhs0, rhs0); ok {
		args := make([]SimpleType, 0, len(lhs.Args))
		argPairs := fun.ZipIter(slices.Values(lhs.Args), slices.Values(rhs.Args))
		for larg, rarg := range argPairs {
			arg, err := lub(larg, rarg)
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
	} else if lhs, rhs, ok := matchPair[Record, Record](lhs0, rhs0); ok {
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
	} else if lhs, rhs, ok := matchPair[ObjectType, ObjectType](lhs0, rhs0); ok {
		lhsMap := namedMembersToMap(lhs.Methods)
		rhsMap := namedMembersToMap(rhs.Methods)

		mergedMap := maps.Clone(lhsMap)
		for rhsKey, rhsVal := range rhsMap {
			if lhsVal, ok := mergedMap[rhsKey]; ok {
				// TODO: visibility
				ty, err := glb(lhsVal.Type, rhsVal.Type)
				if err != nil {
					return nil, err
				}
				mergedMap[rhsKey] = Member{
					Type:   ty,
					Access: types.AccessPublic,
				}
			}
		}
		// TODO: named classes
		return ObjectType{
			Name:    "",
			Supers:  []ObjectType{},
			Fields:  []NamedMember{},
			Methods: mapToNamedMembers(mergedMap),
		}, nil
	} else if _, _, ok := matchPair[Bool, Bool](lhs0, rhs0); ok {
		return Bool{}, nil
	} else if _, _, ok := matchPair[Int, Int](lhs0, rhs0); ok {
		return Int{}, nil
	} else if _, _, ok := matchPair[Str, Str](lhs0, rhs0); ok {
		return Str{}, nil
	} else {
		return Bot{}, nil
	}
}

func lubConcrete(lhs0 ConcreteType, rhs0 ConcreteType) (ConcreteType, error) {
	type C = ConcreteType
	if _, rhs, ok := matchPair[Bot, C](lhs0, rhs0); ok {
		return rhs, nil
	} else if lhs, _, ok := matchPair[C, Bot](lhs0, rhs0); ok {
		return lhs, nil
	} else if _, _, ok := matchPair[Top, C](lhs0, rhs0); ok {
		return Top{}, nil
	} else if _, _, ok := matchPair[C, Top](lhs0, rhs0); ok {
		return Top{}, nil
	} else if lhs, rhs, ok := matchPair[Func, Func](lhs0, rhs0); ok {
		assert.Eq(len(lhs.Args), len(rhs.Args), "different arg count")

		args := make([]SimpleType, 0, len(lhs.Args))
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
	} else if lhs, rhs, ok := matchPair[Record, Record](lhs0, rhs0); ok {
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
	} else if lhs, rhs, ok := matchPair[ObjectType, ObjectType](lhs0, rhs0); ok {
		rhsMap := namedMembersToMap(rhs.Methods)

		merged := []NamedMember{}
		for _, lhsMember := range lhs.Methods {
			if rhsMember, ok := rhsMap[lhsMember.Name]; ok {
				ty, err := lub(lhsMember.Type, rhsMember.Type)
				if err != nil {
					return nil, err
				}

				merged = append(merged, NamedMember{
					Name: lhsMember.Name,
					Member: Member{
						Type:   ty,
						Access: types.AccessPublic, // TODO
					},
				})
			}
		}
		return ObjectType{
			Name:    "",
			Supers:  []ObjectType{},  // TODO
			Fields:  []NamedMember{}, // TODO
			Methods: merged,
		}, nil
	} else if _, _, ok := matchPair[Bool, Bool](lhs0, rhs0); ok {
		return Bool{}, nil
	} else if _, _, ok := matchPair[Int, Int](lhs0, rhs0); ok {
		return Int{}, nil
	} else if _, _, ok := matchPair[Str, Str](lhs0, rhs0); ok {
		return Str{}, nil
	} else {
		return nil, fmt.Errorf("%w: %#v and %#v", ErrIncompatibleTypes, lhs0, rhs0)
	}
}

func glb(lhs0 SimpleType, rhs0 SimpleType) (SimpleType, error) {
	if lhs, rhs, ok := matchPair[ConcreteType, ConcreteType](lhs0, rhs0); ok {
		return glbConcrete(lhs, rhs)
	} else if lhs, rhs, ok := matchPair[*Variable, *Variable](lhs0, rhs0); ok {
		if err := unify(lhs, rhs); err != nil {
			return nil, err
		}

		return rhs, nil
	} else if lhs, rhs, ok := matchPair[ConcreteType, *Variable](lhs0, rhs0); ok {
		if err := rhs.newUpperBound(lhs); err != nil {
			return nil, err
		}

		return rhs, nil
	} else if lhs, rhs, ok := matchPair[*Variable, ConcreteType](lhs0, rhs0); ok {
		if err := lhs.newUpperBound(rhs); err != nil {
			return nil, err
		}

		return rhs, nil
	}
	panic("unreachable")
}
func lub(lhs0 SimpleType, rhs0 SimpleType) (SimpleType, error) {
	if lhs, rhs, ok := matchPair[ConcreteType, ConcreteType](lhs0, rhs0); ok {
		return lubConcrete(lhs, rhs)

	} else if lhs, rhs, ok := matchPair[*Variable, *Variable](lhs0, rhs0); ok {
		if err := unify(lhs, rhs); err != nil {
			return nil, err
		}

		return lhs, nil
	} else if lhs, rhs, ok := matchPair[ConcreteType, *Variable](lhs0, rhs0); ok {
		if err := rhs.newLowerBound(lhs); err != nil {
			return nil, err
		}

		return rhs, nil
	} else if lhs, rhs, ok := matchPair[*Variable, ConcreteType](lhs0, rhs0); ok {
		if err := lhs.newLowerBound(rhs); err != nil {
			return nil, err
		}

		return lhs, nil
	}
	panic(fmt.Sprintf("unreachable: type pair %T, %T", lhs0, rhs0))
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
type Int struct{}
type Str struct{}
type ObjectType struct {
	Name    string
	Supers  []ObjectType
	Fields  []NamedMember
	Methods []NamedMember
}

func (self Top) instantiate() SimpleType        { return self }
func (self Bot) instantiate() SimpleType        { return self }
func (self Func) instantiate() SimpleType       { return self }
func (self Record) instantiate() SimpleType     { return self }
func (self ObjectType) instantiate() SimpleType { return self }
func (self Bool) instantiate() SimpleType       { return self }
func (self Int) instantiate() SimpleType        { return self }
func (self Str) instantiate() SimpleType        { return self }

func (self Top) children() []SimpleType { return []SimpleType{} }
func (self Bot) children() []SimpleType { return []SimpleType{} }
func (self Func) children() []SimpleType {
	return append(append([]SimpleType{}, self.Args...), self.Ret)
}
func (self Record) children() []SimpleType {
	return fun.Map(self.Fields, func(field NamedType) SimpleType { return field.Type })
}
func (self ObjectType) children() []SimpleType {
	c := make([]SimpleType, 0, len(self.Fields)+len(self.Methods))
	for _, field := range self.Fields {
		c = append(c, field.Type)
	}
	for _, meth := range self.Methods {
		c = append(c, meth.Type)
	}
	return c
}
func (self Bool) children() []SimpleType { return []SimpleType{} }
func (self Int) children() []SimpleType  { return []SimpleType{} }
func (self Str) children() []SimpleType  { return []SimpleType{} }

func (self Top) concrete()        {}
func (self Bot) concrete()        {}
func (self Func) concrete()       {}
func (self Record) concrete()     {}
func (self ObjectType) concrete() {}
func (self Bool) concrete()       {}
func (self Int) concrete()        {}
func (self Str) concrete()        {}

func (self Top) String() string { return "⊤" }
func (self Bot) String() string { return "⊥" }
func (self Func) String() string {
	return fmt.Sprintf("(%s -> %s)",
		strings.Join(fun.Map(self.Args, func(t SimpleType) string { return t.String() }), ", "),
		self.Ret.String(),
	)
}
func (self Record) String() string {
	return fmt.Sprintf("{ %s }", strings.Join(fun.Map(self.Fields, func(field NamedType) string {
		return fmt.Sprintf("%s: %s", field.Name, field.Type.String())
	}), ", "))
}
func (self ObjectType) String() string {
	name := self.Name
	if name == "" {
		name = "_UnknownClass"
	}
	return fmt.Sprintf("%s { %s, %s }",
		name,
		strings.Join(fun.Map(self.Fields, func(field NamedMember) string {
			return fmt.Sprintf("%s: %s", field.Name, field.Type.String())
		}), ", "),
		strings.Join(fun.Map(self.Methods, func(field NamedMember) string {
			return fmt.Sprintf("%s: %s", field.Name, field.Type.String())
		}), ", "),
	)
}
func (self Bool) String() string { return "Bool" }
func (self Int) String() string  { return "Int" }
func (self Str) String() string  { return "Str" }

func (self ObjectType) FindMethod(name string) (method Member, found bool) {
	classes := []ObjectType{self}
	for len(classes) > 0 {
		c, ok := popSlice(&classes).Unwrap()
		assert.True(ok, "popSlice returned nothing despite len check")

		for _, m := range c.Methods {
			if m.Name == name {
				return m.Member, true
			}
		}
	}

	return Member{}, false
}

func getVars(ty SimpleType) *orderedset.OrderedSet[*Variable] {
	result := orderedset.NewOrderedSet[*Variable]()
	work := []SimpleType{ty}

	for len(work) > 0 {
		ty, ok := popSlice(&work).Unwrap()
		assert.True(ok, "pop returned empty despite len check")

		if v, ok := ty.(*Variable); ok {
			if result.Has(v) {
				continue
			}
			result.Insert(v)
			work = append(work, v.children()...)
			continue
		}
		work = append(work, ty.children()...)

	}

	return result
}

type SimpleTypeImpl struct{}

// helpers

type Member struct {
	Type   SimpleType
	Access types.AccessLvl
}

type NamedMember struct {
	Name string
	Member
}

type NamedType struct {
	Name string
	Type SimpleType
}

type unit struct{}

// same as concreteEq but takes SimpleType as input and rejects non-ConcreteType
func concreteEq_(lhs, rhs SimpleType) bool {
	if lhs, rhs, ok := matchPair[ConcreteType, ConcreteType](lhs, rhs); ok {
		return concreteEq(lhs, rhs)
	}
	return false
}

func concreteEq(lhs, rhs ConcreteType) bool {
	if reflect.TypeOf(lhs) != reflect.TypeOf(rhs) {
		return false
	}

	if _, _, ok := matchPair[Top, Top](lhs, rhs); ok {
		return true
	} else if _, _, ok := matchPair[Bot, Bot](lhs, rhs); ok {
		return true
	} else if _, _, ok := matchPair[Bool, Bool](lhs, rhs); ok {
		return true
	} else if _, _, ok := matchPair[Int, Int](lhs, rhs); ok {
		return true
	} else if _, _, ok := matchPair[Str, Str](lhs, rhs); ok {
		return true
	} else if left, right, ok := matchPair[Record, Record](lhs, rhs); ok {
		if len(left.Fields) != len(right.Fields) {
			return false
		}

		rightFields := namedTypesToMap(right.Fields)
		for _, field := range left.Fields {
			var rightField SimpleType
			var ok bool
			if rightField, ok = rightFields[field.Name]; !ok {
				return false
			}

			if !concreteEq_(field.Type, rightField) {
				return false
			}
		}
		return true
	} else if left, right, ok := matchPair[Func, Func](lhs, rhs); ok {
		if len(left.Args) != len(right.Args) {
			return false
		}

		for larg, rarg := range fun.ZipIter(slices.Values(left.Args), slices.Values(right.Args)) {
			if !concreteEq_(larg, rarg) {
				return false
			}
		}

		if !concreteEq_(left.Ret, right.Ret) {
			return false
		}
	}

	panic(fmt.Sprintf("compiler bug: concreteEq not implemented for type %T == %T", lhs, rhs))
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

func namedTypesToMap(fields []NamedType) map[string]SimpleType {
	m := map[string]SimpleType{}
	for _, field := range fields {
		m[field.Name] = field.Type
	}
	return m
}

func namedMembersToMap(members []NamedMember) map[string]Member {
	m := map[string]Member{}
	for _, member := range members {
		m[member.Name] = member.Member
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

func mapToNamedMembers(fields map[string]Member) []NamedMember {
	s := make([]NamedMember, 0, len(fields))
	for k, v := range fields {
		s = append(s, NamedMember{k, v})
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

var gIdCounter uint = 0

func newId() uint {
	gIdCounter++
	return gIdCounter
}

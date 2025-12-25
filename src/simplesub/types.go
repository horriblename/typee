// # Type Hierarchy
//
// There are 3 overarching types: [TypeScheme], [SimpleType], and
// [ConcreteType].
//
//	       ┌──────────┐
//	       │TypeScheme│
//	       └──────────┘
//	            ▲
//	      ┌─────┴────────┐
//	      │         ┌────┴─────┐
//	PolymorphicType │SimpleType│
//	                └──────────┘
//	                     ▲
//	             ┌───────┴────────┐
//	             │          ┌────────────┐
//	          Variable      │ConcreteType│
//	                        └────────────┘
//	                              ▲
//	                              └─ Top, Bot, Int, Record, ...
package simplesub

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/can"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/internal/ordered_set"
	"github.com/horriblename/typee/src/opt"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/types"
)

//go-sumtype:decl TypeScheme SimpleType ConcreteType

type PrimitiveKind string

const (
	PrimitiveBool   PrimitiveKind = "Bool"
	PrimitiveFloat  PrimitiveKind = "Float"
	PrimitiveOpaque PrimitiveKind = "Opaque"
)

var ErrInvalidCyclicConstraint = errors.New("invalid cyclic constraint")
var ErrIncompatibleTypes = errors.New("incompatible types")
var ErrIncompatibleTag = errors.New("incompatible tag")
var ErrWrongTypeParamCount = errors.New("wrong type parameter count")

// TypeScheme is a type that potentially contains universally quantified type variables.
// can be instantiated to a given level
type TypeScheme interface {
	instantiate() SimpleType
	String() string
}

// PolymorphicType is a type with universally quantified type variables
type PolymorphicType struct {
	Body       SimpleType
	TypeParams opt.Option[[]uint] // list of quantified variable IDs
}

func (self PolymorphicType) instantiate() SimpleType {
	return freshenType(self.Body)
}

// instantiate only type vars in [PolymorphicType.TypeParams]
func (self PolymorphicType) concretize(params []SimpleType) (SimpleType, error) {
	mappings := map[uint]SimpleType{}
	if len(params) != len(self.TypeParams.Or([]uint{})) {
		return nil, fmt.Errorf("%w: expected %d, got %d",
			ErrWrongTypeParamCount,
			len(params),
			len(self.TypeParams.Or([]uint{})),
		)
	}

	if typeParams, ok := self.TypeParams.Unwrap(); ok {
		for i, p := range params {
			mappings[typeParams[i]] = p
		}
		return concretizeType(self.Body, mappings), nil
	}
	return self.Body, nil
}

func (self PolymorphicType) String() string {
	return fmt.Sprintf("polymorphic{%s}", self.Body.String())
}

// SimpleType is a type without universally quantified type variables
type SimpleType interface {
	TypeScheme
	children() []SimpleType
}

type Variable struct {
	uid            uint
	lowerBound     ConcreteType
	upperBound     ConcreteType
	representative *Variable // nilable
}

func (self *Variable) instantiate() SimpleType { return self }
func (self *Variable) children() []SimpleType {
	return []SimpleType{self.LowerBound(), self.UpperBound()}
}
func (self *Variable) Uid() uint {
	return self.Representative().uid
}
func (self *Variable) asTypeVar() types.Type {
	return &types.Generic{ID: types.TypeID(self.uid)}
}

func (self *Variable) String() string {
	if self.representative != nil {
		return fmt.Sprintf("t%d=t%d[%v, %v]", self.uid, self.representative.uid, self.LowerBound(), self.UpperBound())
	}
	return fmt.Sprintf("t%d[%v, %v]", self.uid, self.lowerBound, self.upperBound)
}
func (self *Variable) LowerBound() ConcreteType { return self.Representative().lowerBound }
func (self *Variable) UpperBound() ConcreteType { return self.Representative().upperBound }
func (self *Variable) newUpperBound(symbols *symbols, ub ConcreteType) error {
	if err := self.occursCheck(ub, true); err != nil {
		return err
	}

	rep := self.Representative()
	newUb, err := symbols.glbConcrete(rep.upperBound, ub)
	if err != nil {
		return err
	}
	rep.upperBound = newUb

	trace("new upper bound for t%d: %v", self.uid, newUb)
	indentLvl++
	defer func() { indentLvl-- }()
	return symbols.constrain(rep.lowerBound, ub)
}
func (self *Variable) newLowerBound(symbols *symbols, lb ConcreteType) error {
	var err error
	if err := self.occursCheck(lb, false); err != nil {
		return err
	}

	rep := self.Representative()

	rep.lowerBound, err = symbols.lubConcrete(rep.lowerBound, lb)
	if err != nil {
		return err
	}

	trace("new lower bound for t%d: %v", self.uid, rep.lowerBound)
	indentLvl++
	defer func() { indentLvl-- }()
	return symbols.constrain(lb, rep.upperBound)
}
func (self *Variable) Representative() *Variable {
	if self.representative != nil {
		rep := self.representative.Representative()
		if rep != self {
			self.representative = rep
		}
		return rep
	} else {
		return self
	}
}

func (self *Variable) occursCheck(ty ConcreteType, dir bool) error {
	// TODO: fail early instead of search everything in getVars
	if getVars(ty).Has(self.Representative()) {
		relation := ":>"
		if dir {
			relation = "<:"
		}
		return fmt.Errorf("%w: %v %s %v", ErrInvalidCyclicConstraint, self, relation, ty)
	}
	return nil
}

func (self *symbols) glbConcrete(lhs0 ConcreteType, rhs0 ConcreteType) (ConcreteType, error) {
	type C = ConcreteType
	if _, rhs, ok := matchPair[Top, C](lhs0, rhs0); ok {
		return rhs, nil
	} else if lhs, _, ok := matchPair[C, Top](lhs0, rhs0); ok {
		return lhs, nil
	} else if _, rhs, ok := matchPair[Bot, C](lhs0, rhs0); ok {
		return rhs, nil
	} else if lhs, _, ok := matchPair[C, Bot](lhs0, rhs0); ok {
		return lhs, nil
	} else if lhs, rhs, ok := matchPair[Application, Application](lhs0, rhs0); ok {
		if lhs.Module == rhs.Module && lhs.Name == rhs.Name {
			return lhs, nil
		}

		lhsCon, err := self.concretizeApplication(lhs)
		if err != nil {
			return nil, err
		}

		rhsCon, err := self.concretizeApplication(rhs)
		if err != nil {
			return nil, err
		}

		return self.glbConcrete(lhsCon, rhsCon)
	} else if lhs, rhs, ok := matchPair[C, Application](lhs0, rhs0); ok {
		return self.glbConcrete(rhs, lhs)
	} else if lhs, rhs, ok := matchPair[Application, C](lhs0, rhs0); ok {
		lhsCon, err := self.concretizeApplication(lhs)
		if err != nil {
			return nil, err
		}

		return self.glbConcrete(lhsCon, rhs)
	} else if lhs, rhs, ok := matchPair[Func, Func](lhs0, rhs0); ok {
		args := make([]SimpleType, 0, len(lhs.Args))
		assert.Eq(len(lhs.Args), len(rhs.Args),
			"BUG mismatched arg count detected in glb for types: ", lhs, rhs)

		argPairs := fun.ZipIterStrict(slices.Values(lhs.Args), slices.Values(rhs.Args))
		for larg, rarg := range argPairs {
			arg, err := self.lub(larg, rarg)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}

		ret, err := self.glb(lhs.Ret, rhs.Ret)
		if err != nil {
			return nil, err
		}

		assert.Eq(lhs.Method, rhs.Method, "typer BUG mismatched Func.Method in glb?")
		return Func{args, ret, lhs.Method}, nil
	} else if lhs, rhs, ok := matchPair[Record, Record](lhs0, rhs0); ok {
		var err error
		lhsMap := namedTypesToMap(lhs.Fields)
		rhsMap := namedTypesToMap(rhs.Fields)

		mergedMap := maps.Clone(lhsMap)
		for rhsKey, rhsVal := range rhsMap {
			if lhsVal, ok := mergedMap[rhsKey]; ok {
				mergedMap[rhsKey], err = self.glb(lhsVal, rhsVal)
				if err != nil {
					return nil, err
				}

			}
		}
		return Record{mapToNamedTypes(mergedMap)}, nil
	} else if lhs, rhs, ok := matchPair[Record, ObjectType](lhs0, rhs0); ok {
		return self.glbCrossObject(lhs, rhs)
	} else if lhs, rhs, ok := matchPair[ObjectType, Record](lhs0, rhs0); ok {
		return self.glbCrossObject(rhs, lhs)
	} else if lhs, rhs, ok := matchPair[ObjectType, ObjectType](lhs0, rhs0); ok {
		// TODO: might be better to reformulate glb(ObjectType, ObjectType) in the
		// same way as record types: i.e. glb(A, B) is an unnamed class with all
		// the fields & methods of A & B, glb'd if both classes contain a
		// field/method of the same name. (should also assert that method and field
		// names don't conflict)
		//
		// Then we can maybe add special cases for named types:
		// (these are all still pretty expensive, especially considering
		// interfaces)
		// - if namedA <: namedB then glb(namedA, namedB) = namedA
		// - same for namedB <: namedA
		// - otherwise, we walk up type hierarchy until a common supertype is found?
		//   (would benefit from ID'd types - assign IDs to classes in such a way
		//   that supertype IDs are always less than subtype IDs - some care should
		//   be taken in regards to imports, to ensure modules can be processed in
		//   parallel)
		if lhs.Name != "" && rhs.Name != "" {
			return self.glbNamedObjects(lhs, rhs)
		}

		// return the named class if named <: unnamed
		if lhs.Name != "" {
			if err := self.constrain(lhs, rhs); err != nil {
				// TODO: how should I handle this
				return nil, fmt.Errorf("unimplemented: glb(unnamed_class, named_class): constrain result: %v", err)
			}

			return lhs, nil
		}

		// return the named class if named <: unnamed
		if rhs.Name != "" {
			if err := self.constrain(rhs, lhs); err != nil {
				// TODO: how should I handle this
				return nil, fmt.Errorf("unimplemented: glb(named_class, unnamed_class): constrain result: %v", err)
			}
			return rhs, nil
		}

		return self.glbUnnamedObjects(lhs, rhs)
	} else if lhs, rhs, ok := matchPair[ArrayType, ArrayType](lhs0, rhs0); ok {
		lb, err := self.glb(lhs.ElType, rhs.ElType)
		if err != nil {
			return nil, err
		}

		if lhs.Size != rhs.Size {
			return nil, fmt.Errorf("%w: size mismatch of arrays %s != %s", ErrIncompatibleTypes, lhs.String(), rhs.String())
		}

		return ArrayType{lb, lhs.Size}, nil
	} else if lhs, rhs, ok := matchPair[SliceType, SliceType](lhs0, rhs0); ok {
		lb, err := self.glb(lhs.ElType, rhs.ElType)
		if err != nil {
			return nil, err
		}

		return SliceType{lb}, nil
	} else if lhs, rhs, ok := matchPair[Union, Union](lhs0, rhs0); ok {
		lhsSet := sliceToSet(lhs.Variants)
		intersection := map[ConcreteType]struct{}{}
		for _, rhsKey := range rhs.Variants {
			if _, ok := lhsSet[rhsKey]; ok {
				intersection[rhsKey] = struct{}{}
			}
		}

		return Union{"", setToSlice(intersection)}, nil
	} else if lhs, rhs, ok := matchPair[ConcreteType, Union](lhs0, rhs0); ok {
		for _, variant := range rhs.Variants {
			// TODO: should probably be a constrain to represent lhs <: variant
			if concreteEq(lhs, variant) {
				return rhs, nil
			}
		}

		return nil, fmt.Errorf("%w: %s cannot be used as a %s and vice versa", ErrIncompatibleTypes, lhs, rhs)
	} else if lhs, rhs, ok := matchPair[Union, ConcreteType](lhs0, rhs0); ok {
		for _, variant := range lhs.Variants {
			// TODO: should probably be a constrain to represent rhs <: variant
			if concreteEq(rhs, variant) {
				return lhs, nil
			}
		}

		return nil, fmt.Errorf("%w: %s cannot be used as a %s and vice versa", ErrIncompatibleTypes, rhs, lhs)
	} else if lhs, rhs, ok := matchPair[Primitive, Primitive](lhs0, rhs0); ok {
		if lhs.Kind == rhs.Kind {
			return Primitive{lhs.Kind}, nil
		}

		return nil, fmt.Errorf("%w: %s and %s", ErrIncompatibleTypes, lhs, rhs)
	} else if lhs, rhs, ok := matchPair[Int, Int](lhs0, rhs0); ok {
		if lhs.Signed != rhs.Signed || lhs.BitSize != rhs.BitSize {
			return nil, fmt.Errorf("%w: integer conversion not implemented yet", ErrIncompatibleTypes)
		}
		return lhs, nil
	} else if _, _, ok := matchPair[Str, Str](lhs0, rhs0); ok {
		return Str{}, nil
	} else if lhs, rhs, ok := matchPair[Ref, Ref](lhs0, rhs0); ok {
		content, err := self.glb(lhs.Content, rhs.Content)
		if err != nil {
			return nil, err
		}

		return Ref{content}, nil
	} else if lhs, rhs, ok := matchPair[Ref, Primitive](lhs0, rhs0); ok {
		if rhs.Kind == PrimitiveOpaque {
			return lhs, nil
		}
		return nil, fmt.Errorf("%w: %s and %s", ErrIncompatibleTypes, lhs0, rhs0)
	} else if lhs, _, ok := matchPair[Primitive, Ref](lhs0, rhs0); ok {
		if lhs.Kind == PrimitiveOpaque {
			return rhs, nil
		}
		return nil, fmt.Errorf("%w: %s and %s", ErrIncompatibleTypes, lhs0, rhs0)
	} else if _, rhs, ok := matchPair[ObjectType, Primitive](lhs0, rhs0); ok {
		if rhs.Kind == PrimitiveOpaque {
			return rhs, nil
		}
		return nil, fmt.Errorf("%w: %s and %s", ErrIncompatibleTypes, lhs0, rhs0)
	} else if lhs, _, ok := matchPair[Primitive, ObjectType](lhs0, rhs0); ok {
		if lhs.Kind == PrimitiveOpaque {
			return lhs, nil
		}
		return nil, fmt.Errorf("%w: %s and %s", ErrIncompatibleTypes, lhs0, rhs0)
	} else if lhs, rhs, ok := matchPair[TaggedUnion, TaggedUnion](lhs0, rhs0); ok {
		inter := map[string]opt.Option[SimpleType]{}
		for tag, lvar := range lhs.Variants {
			if rvar, ok := rhs.Variants[tag]; ok {
				lv, lok := lvar.Unwrap()
				rv, rok := rvar.Unwrap()

				if lok && rok {
					lub, err := self.lub(lv, rv)
					if err != nil {
						return nil, fmt.Errorf("%w %s: %w",
							ErrIncompatibleTag, tag, err)
					}

					inter[tag] = opt.Some(lub)
				} else if lok != rok {
					return nil, fmt.Errorf(
						"%w %s: one side has payload the other doesn't: %v and %v",
						ErrIncompatibleTag, tag, lvar, rvar)
				}
			}
		}
		return TaggedUnion{
			Name:     "", // FIXME: put something
			Variants: inter,
		}, nil
	} else if lhs, rhs, ok := matchPair[Enum, Enum](lhs0, rhs0); ok {
		// same non-empty name
		if lhs.Name == rhs.Name && lhs.Name != "" {
			return lhs, nil
		}

		// different non-empty name
		if lhs.Name != "" && rhs.Name != "" {
			return nil, fmt.Errorf("different enum types: %s and %s", lhs.Name, rhs.Name)
		}

		// FIXME: at least check that unnamed enum values are all present in named one
		if lhs.Name != "" {
			return lhs, nil
		} else if rhs.Name != "" {
			return rhs, nil
		}

		panic("TODO: glb of unnamed enums")

		// TODO: lower bound of [Enum, Int]?
	} else {
		return nil, fmt.Errorf("%w: %s and %s", ErrIncompatibleTypes, lhs0, rhs0)
	}
}

func (self *symbols) glbUnnamedObjects(lhs ObjectType, rhs ObjectType) (ConcreteType, error) {
	assert.Eq(lhs.Name, "", "compiler invariant broken: lubUnnamedObjects called on ", lhs.Module, ".", lhs.Name)
	assert.Eq(rhs.Name, "", "compiler invariant broken: lubUnnamedObjects called on ", rhs.Module, ".", rhs.Name)

	lhsFields := namedMembersToMap(lhs.Fields)
	rhsFields := namedMembersToMap(rhs.Fields)

	mergedFields := maps.Clone(lhsFields)
	for rhsKey, rhsVal := range rhsFields {
		if lhsVal, ok := mergedFields[rhsKey]; ok {
			ty, err := self.glb(lhsVal.Type, rhsVal.Type)
			if err != nil {
				return nil, err
			}
			mergedFields[rhsKey] = Member{
				Type:   ty,
				Access: max(lhsVal.Access, rhsVal.Access),
			}
		}
	}

	lhsMap := namedMembersToMap(lhs.Methods)
	rhsMap := namedMembersToMap(rhs.Methods)

	mergedMeths := maps.Clone(lhsMap)
	for rhsKey, rhsVal := range rhsMap {
		if lhsVal, ok := mergedMeths[rhsKey]; ok {
			ty, err := self.glb(lhsVal.Type, rhsVal.Type)
			if err != nil {
				return nil, err
			}
			mergedMeths[rhsKey] = Member{
				Type:   ty,
				Access: max(lhsVal.Access, rhsVal.Access),
			}
		}
	}

	return ObjectType{
		Module: "",
		Name:   "",
		// FIXME: what should kind be?
		Supers:  []Application{},
		Fields:  mapToNamedMembers(mergedFields),
		Methods: mapToNamedMembers(mergedMeths),
	}, nil
}

func (self *symbols) glbCrossObject(lhs Record, rhs ObjectType) (ConcreteType, error) {
	var err error
	lhsMap := namedTypesToMap(lhs.Fields)
	rhsMap := namedMembersToMap(rhs.Fields)

	mergedMap := maps.Clone(lhsMap)
	for rhsKey, rhsVal := range rhsMap {
		if lhsVal, ok := mergedMap[rhsKey]; ok {
			mergedMap[rhsKey], err = self.glb(lhsVal, rhsVal.Type)
			if err != nil {
				return nil, err
			}
		}
	}
	return Record{mapToNamedTypes(mergedMap)}, nil
}

func (self *symbols) glbNamedObjects(lhs ObjectType, rhs ObjectType) (ObjectType, error) {
	assert.Neq(lhs.Name, "", "BUG typer: glbNamedObjects called on unnamed object")
	assert.Neq(rhs.Name, "", "BUG typer: glbNamedObjects called on unnamed object")

	if lhs.Module == rhs.Module && lhs.Name == rhs.Name {
		return lhs, nil
	} else if lhs.Module == self.mainModule && rhs.Module != self.mainModule {
		return lhs, nil
	} else if lhs.Module != self.mainModule && rhs.Module == self.mainModule {
		return rhs, nil
	}

	if ok, err := self.isSubClass(lhs, rhs); err != nil {
		return ObjectType{}, fmt.Errorf("%w: finding glb of %s and %s", err, lhs, rhs)
	} else if ok {
		return lhs, nil
	}

	if ok, err := self.isSubClass(rhs, lhs); err != nil {
		return ObjectType{}, fmt.Errorf("%w: finding glb of %s and %s", err, lhs, rhs)
	} else if ok {
		return rhs, nil
	}

	return ObjectType{}, fmt.Errorf("%w %s and %s (or I haven't implemented proper class glb)",
		ErrTypeMismatch, lhs, rhs)
}

func (self *symbols) lubConcrete(lhs0 ConcreteType, rhs0 ConcreteType) (ConcreteType, error) {
	type C = ConcreteType
	if _, rhs, ok := matchPair[Bot, C](lhs0, rhs0); ok {
		return rhs, nil
	} else if lhs, _, ok := matchPair[C, Bot](lhs0, rhs0); ok {
		return lhs, nil
	} else if _, _, ok := matchPair[Top, C](lhs0, rhs0); ok {
		return Top{}, nil
	} else if _, _, ok := matchPair[C, Top](lhs0, rhs0); ok {
		return Top{}, nil
	} else if lhs, rhs, ok := matchPair[Application, Application](lhs0, rhs0); ok {
		if lhs.Module == rhs.Module && lhs.Name == rhs.Name {
			return lhs, nil
		}

		lhsCon, err := self.concretizeApplication(lhs)
		if err != nil {
			return nil, err
		}

		rhsCon, err := self.concretizeApplication(rhs)
		if err != nil {
			return nil, err
		}

		return self.lubConcrete(lhsCon, rhsCon)
	} else if lhs, rhs, ok := matchPair[C, Application](lhs0, rhs0); ok {
		return self.lubConcrete(rhs, lhs)
	} else if lhs, rhs, ok := matchPair[Application, C](lhs0, rhs0); ok {
		lhsCon, err := self.concretizeApplication(lhs)
		if err != nil {
			return nil, err
		}

		return self.lubConcrete(lhsCon, rhs)
	} else if lhs, rhs, ok := matchPair[Func, Func](lhs0, rhs0); ok {
		assert.Eq(len(lhs.Args), len(rhs.Args), "different arg count")

		args := make([]SimpleType, 0, len(lhs.Args))
		for i, lhsArg := range lhs.Args {
			ty, err := self.glb(lhsArg, rhs.Args[i])
			if err != nil {
				return nil, err
			}
			args = append(args, ty)
		}

		ret, err := self.lub(lhs.Ret, rhs.Ret)
		if err != nil {
			return nil, err
		}

		assert.Eq(lhs.Method, rhs.Method, "typer BUG mismatched Func.Method in lub?")
		return Func{args, ret, lhs.Method}, nil
	} else if lhs, rhs, ok := matchPair[Record, Record](lhs0, rhs0); ok {
		// the "intersection" of both records
		rhsMap := namedTypesToMap(rhs.Fields)

		merged := []NamedType{}
		for _, lhsField := range lhs.Fields {
			if rhsField, ok := rhsMap[lhsField.Name]; ok {
				// TODO: reject "union" types like int|string
				ty, err := self.lub(lhsField.Type, rhsField)
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
	} else if lhs, rhs, ok := matchPair[Record, ObjectType](lhs0, rhs0); ok {
		return self.lubCrossObject(lhs, rhs)
	} else if lhs, rhs, ok := matchPair[ObjectType, Record](lhs0, rhs0); ok {
		return self.lubCrossObject(rhs, lhs)
	} else if lhs, rhs, ok := matchPair[ArrayType, ArrayType](lhs0, rhs0); ok {
		el, err := self.lub(lhs.ElType, rhs.ElType)
		if err != nil {
			return nil, err
		}

		if lhs.Size != rhs.Size {
			return nil, fmt.Errorf("%w: size mismatch of arrays %s and %s", ErrIncompatibleTypes, lhs, rhs)
		}

		return ArrayType{el, lhs.Size}, nil
	} else if lhs, rhs, ok := matchPair[SliceType, SliceType](lhs0, rhs0); ok {
		el, err := self.lub(lhs.ElType, rhs.ElType)
		if err != nil {
			return nil, err
		}

		return SliceType{el}, nil
	} else if lhs, rhs, ok := matchPair[Union, Union](lhs0, rhs0); ok {
		rhsSet := sliceToSet(rhs.Variants)
		merged := make([]ConcreteType, 0, len(rhs.Variants))
		copy(rhs.Variants, merged)

		for _, lhsVariant := range lhs.Variants {
			if _, ok := rhsSet[lhsVariant]; !ok {
				merged = append(merged, lhsVariant)
			}
		}

		return Union{"", merged}, nil
	} else if lhs, rhs, ok := matchPair[ConcreteType, Union](lhs0, rhs0); ok {
		for _, variant := range rhs.Variants {
			// TODO: should probably use constrain to represent lhs <: variant
			if concreteEq(lhs, variant) {
				return lhs, nil
			}
		}

		return nil, fmt.Errorf("%w: %s cannot be used as %s and vice versa", ErrIncompatibleTypes, lhs, rhs)
	} else if lhs, rhs, ok := matchPair[Union, ConcreteType](lhs0, rhs0); ok {
		for _, variant := range lhs.Variants {
			// TODO: should probably use constrain to represent rhs <: variant
			if concreteEq(rhs, variant) {
				return rhs, nil
			}
		}

		return nil, fmt.Errorf("%w: %s cannot be used as %s and vice versa", ErrIncompatibleTypes, rhs, lhs)
	} else if lhs, rhs, ok := matchPair[TaggedUnion, TaggedUnion](lhs0, rhs0); ok {
		// union of both variant sets. overlapping tags are glb'd
		union := map[string]opt.Option[SimpleType]{}
		maps.Copy(union, lhs.Variants)
		for tag, rvar := range rhs.Variants {
			if lvar, ok := union[tag]; ok {
				lv, lok := lvar.Unwrap()
				rv, rok := rvar.Unwrap()
				if lok && rok {
					glb, err := self.glb(lv, rv)
					if err != nil {
						return nil, fmt.Errorf("%w %s: %w",
							ErrIncompatibleTag, tag, err)
					}

					union[tag] = opt.Some(glb)
				} else if lok != rok {
					return nil, fmt.Errorf(
						"%w %s: one side has payload the other doesn't: %v and %v",
						ErrIncompatibleTag, tag, lvar, rvar)
				}
			} else {
				union[tag] = rvar
			}
		}
		return TaggedUnion{
			Name:     "", // FIXME: fuck I was not thinking about this
			Variants: union,
		}, nil
	} else if lhs, rhs, ok := matchPair[Enum, Enum](lhs0, rhs0); ok {
		if lhs.Name != rhs.Name {
			return nil, fmt.Errorf("different enum types: %s and %s", lhs.Name, rhs.Name)
		}

		return lhs, nil
	} else if lhs, rhs, ok := matchPair[ObjectType, ObjectType](lhs0, rhs0); ok {
		if lhs.Name != "" && rhs.Name != "" {
			return self.lubNamedObjects(lhs, rhs)
		}

		// return the named class if named :> unnamed
		if lhs.Name != "" {
			if err := self.constrain(rhs, lhs); err != nil {
				return nil, fmt.Errorf("unimplemented: lub(named_class, unnamed_class): constrain result: %v", err)
			}

			return lhs, nil
		}

		// return the named class if named :> unnamed
		if rhs.Name != "" {
			if err := self.constrain(lhs, rhs); err != nil {
				return nil, fmt.Errorf("unimplemented: lub(named_class, unnamed_class): constrain result: %v", err)
			}

			return rhs, nil
		}

		return self.lubUnnamedObjects(lhs, rhs)
	} else if lhs, rhs, ok := matchPair[Primitive, Primitive](lhs0, rhs0); ok {
		if lhs.Kind == rhs.Kind {
			return Primitive{lhs.Kind}, nil
		}
		return nil, fmt.Errorf("%w: %s and %s", ErrIncompatibleTypes, lhs, rhs)
	} else if lhs, rhs, ok := matchPair[Int, Int](lhs0, rhs0); ok {
		if lhs.Signed != rhs.Signed || lhs.BitSize != rhs.BitSize {
			return nil, fmt.Errorf("%w: integer conversion not implemented yet", ErrIncompatibleTypes)
		}
		return lhs, nil
	} else if _, _, ok := matchPair[Str, Str](lhs0, rhs0); ok {
		return Str{}, nil
	} else if lhs, rhs, ok := matchPair[Ref, Ref](lhs0, rhs0); ok {
		content, err := self.lub(lhs.Content, rhs.Content)
		if err != nil {
			return nil, err
		}

		return Ref{content}, nil
	} else if _, rhs, ok := matchPair[Ref, Primitive](lhs0, rhs0); ok {
		if rhs.Kind == PrimitiveOpaque {
			return rhs, nil
		}
		return nil, fmt.Errorf("%w: %s and %s", ErrIncompatibleTypes, lhs0, rhs0)
	} else if lhs, _, ok := matchPair[Primitive, Ref](lhs0, rhs0); ok {
		if lhs.Kind == PrimitiveOpaque {
			return rhs, nil
		}
		return nil, fmt.Errorf("%w: %s and %s", ErrIncompatibleTypes, lhs0, rhs0)
	} else if lhs, rhs, ok := matchPair[ObjectType, Primitive](lhs0, rhs0); ok {
		if rhs.Kind == PrimitiveOpaque {
			return lhs, nil
		}
		return nil, fmt.Errorf("%w: %s and %s", ErrIncompatibleTypes, lhs0, rhs0)
	} else if lhs, rhs, ok := matchPair[Primitive, ObjectType](lhs0, rhs0); ok {
		if lhs.Kind == PrimitiveOpaque {
			return rhs, nil
		}
		return nil, fmt.Errorf("%w: %s and %s", ErrIncompatibleTypes, lhs0, rhs0)
	} else {
		return nil, fmt.Errorf("%w: %s and %s", ErrIncompatibleTypes, lhs0, rhs0)
	}
}

func (self *symbols) lubUnnamedObjects(lhs ObjectType, rhs ObjectType) (ConcreteType, error) {
	assert.Eq(lhs.Name, "", "compiler invariant broken: lubUnnamedObjects called on ", lhs.Module, ".", lhs.Name)
	assert.Eq(rhs.Name, "", "compiler invariant broken: lubUnnamedObjects called on ", rhs.Module, ".", rhs.Name)

	rhsFieldMap := namedMembersToMap(rhs.Fields)

	fields := []NamedMember{}
	for _, lhsMember := range lhs.Fields {
		if rhsMember, ok := rhsFieldMap[lhsMember.Name]; ok {
			ty, err := self.lub(lhsMember.Type, rhsMember.Type)
			if err != nil {
				return nil, err
			}

			fields = append(fields, NamedMember{
				Name: lhsMember.Name,
				Member: Member{
					Type:   ty,
					Access: min(lhsMember.Access, rhsMember.Access),
				},
			})
		}
	}

	rhsMethMap := namedMembersToMap(rhs.Methods)
	merged := []NamedMember{}
	for _, lhsMember := range lhs.Methods {
		if rhsMember, ok := rhsMethMap[lhsMember.Name]; ok {
			ty, err := self.lub(lhsMember.Type, rhsMember.Type)
			if err != nil {
				return nil, err
			}

			merged = append(merged, NamedMember{
				Name: lhsMember.Name,
				Member: Member{
					Type:   ty,
					Access: min(lhsMember.Access, rhsMember.Access),
				},
			})
		}
	}
	return ObjectType{
		Module: "",
		Name:   "",
		// FIXME: what should Kind be?
		Supers:  []Application{}, // TODO
		Fields:  fields,
		Methods: merged,
	}, nil
}

func (self *symbols) lubCrossObject(lhs Record, rhs ObjectType) (ConcreteType, error) {
	// the "intersection" of both records
	rhsMap := namedMembersToMap(rhs.Fields)

	merged := []NamedType{}
	for _, lhsField := range lhs.Fields {
		if rhsField, ok := rhsMap[lhsField.Name]; ok {
			// TODO: reject "union" types like int|string
			ty, err := self.lub(lhsField.Type, rhsField.Type)
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
}

func (self *symbols) lubNamedObjects(lhs ObjectType, rhs ObjectType) (ObjectType, error) {
	assert.Neq(lhs.Name, "", "BUG typer: lubNamedObjects called on unnamed object")
	assert.Neq(rhs.Name, "", "BUG typer: lubNamedObjects called on unnamed object")

	if lhs.Module == rhs.Module && lhs.Name == rhs.Name {
		return lhs, nil
	} else if lhs.Module == self.mainModule && rhs.Module != self.mainModule {
		return rhs, nil
	} else if lhs.Module != self.mainModule && rhs.Module == self.mainModule {
		return lhs, nil
	}

	if ok, err := self.isSubClass(lhs, rhs); err != nil {
		return ObjectType{}, fmt.Errorf("%w: finding glb of %s and %s", err, lhs, rhs)
	} else if ok {
		return rhs, nil
	}

	if ok, err := self.isSubClass(rhs, lhs); err != nil {
		return ObjectType{}, fmt.Errorf("%w: finding glb of %s and %s", err, lhs, rhs)
	} else if ok {
		return lhs, nil
	}

	return ObjectType{}, fmt.Errorf("%w %s and %s (or I haven't implemented proper class glb)",
		ErrTypeMismatch, lhs, rhs)
}
func (self *symbols) glb(lhs0 SimpleType, rhs0 SimpleType) (SimpleType, error) {
	if lhs, rhs, ok := matchPair[ConcreteType, ConcreteType](lhs0, rhs0); ok {
		return self.glbConcrete(lhs, rhs)
	} else if lhs, rhs, ok := matchPair[*Variable, *Variable](lhs0, rhs0); ok {
		if err := self.unify(lhs, rhs); err != nil {
			return nil, err
		}

		return rhs, nil
	} else if lhs, rhs, ok := matchPair[ConcreteType, *Variable](lhs0, rhs0); ok {
		if err := rhs.newUpperBound(self, lhs); err != nil {
			return nil, err
		}

		return rhs, nil
	} else if lhs, rhs, ok := matchPair[*Variable, ConcreteType](lhs0, rhs0); ok {
		if err := lhs.newUpperBound(self, rhs); err != nil {
			return nil, err
		}

		return rhs, nil
	}
	panic("unreachable")
}
func (self *symbols) lub(lhs0 SimpleType, rhs0 SimpleType) (SimpleType, error) {
	if lhs, rhs, ok := matchPair[ConcreteType, ConcreteType](lhs0, rhs0); ok {
		return self.lubConcrete(lhs, rhs)

	} else if lhs, rhs, ok := matchPair[*Variable, *Variable](lhs0, rhs0); ok {
		if err := self.unify(lhs, rhs); err != nil {
			return nil, err
		}

		return lhs, nil
	} else if lhs, rhs, ok := matchPair[ConcreteType, *Variable](lhs0, rhs0); ok {
		if err := rhs.newLowerBound(self, lhs); err != nil {
			return nil, err
		}

		return rhs, nil
	} else if lhs, rhs, ok := matchPair[*Variable, ConcreteType](lhs0, rhs0); ok {
		if err := lhs.newLowerBound(self, rhs); err != nil {
			return nil, err
		}

		return lhs, nil
	}
	panic(fmt.Sprintf("unreachable: type pair %T, %T", lhs0, rhs0))
}

func matchPair[L, R any](lhs any, rhs any) (_ L, _ R, ok bool) {
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
	Args   []SimpleType
	Ret    SimpleType
	Method bool
}
type Record struct{ Fields []NamedType }
type Primitive struct{ Kind PrimitiveKind }
type Int struct {
	Signed  bool
	BitSize int64
}
type Str struct{}
type Ref struct{ Content SimpleType }
type Union struct {
	Name     string
	Variants []ConcreteType
}
type Enum struct {
	Name   string
	Values map[string]opt.Option[int64]
}
type TaggedUnion struct {
	Name     string
	Variants map[string]opt.Option[SimpleType]
}
type ObjectType struct {
	Module  can.ModuleName
	Kind    parse.ObjectKind
	Name    string
	Supers  []Application
	Fields  []NamedMember
	Methods []NamedMember
	Top     bool // top class is a class that does not have a parent class
}
type ArrayType struct {
	ElType SimpleType
	Size   uint
}
type SliceType struct {
	ElType SimpleType
}
type Application struct {
	Module can.ModuleName
	Name   string
	Params []SimpleType
}

func (self Top) instantiate() SimpleType         { return self }
func (self Bot) instantiate() SimpleType         { return self }
func (self Func) instantiate() SimpleType        { return self }
func (self Record) instantiate() SimpleType      { return self }
func (self ObjectType) instantiate() SimpleType  { return self }
func (self ArrayType) instantiate() SimpleType   { return self }
func (self SliceType) instantiate() SimpleType   { return self }
func (self Primitive) instantiate() SimpleType   { return self }
func (self Int) instantiate() SimpleType         { return self }
func (self Str) instantiate() SimpleType         { return self }
func (self Ref) instantiate() SimpleType         { return self }
func (self Union) instantiate() SimpleType       { return self }
func (self Enum) instantiate() SimpleType        { return self }
func (self TaggedUnion) instantiate() SimpleType { return self }
func (self Application) instantiate() SimpleType {
	return Application{
		Module: self.Module,
		Name:   self.Name,
		Params: fun.Map(self.Params, func(t SimpleType) SimpleType {
			return t.instantiate()
		}),
	}
}

func (self Top) children() []SimpleType { return []SimpleType{} }
func (self Bot) children() []SimpleType { return []SimpleType{} }
func (self Func) children() []SimpleType {
	return append(slices.Clone(self.Args), self.Ret)
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
func (self ArrayType) children() []SimpleType { return []SimpleType{self.ElType} }
func (self SliceType) children() []SimpleType { return []SimpleType{self.ElType} }
func (self Primitive) children() []SimpleType { return []SimpleType{} }
func (self Int) children() []SimpleType       { return []SimpleType{} }
func (self Str) children() []SimpleType       { return []SimpleType{} }
func (self Ref) children() []SimpleType       { return []SimpleType{} }
func (self Union) children() []SimpleType {
	return fun.Map(self.Variants, func(c ConcreteType) SimpleType { return c })
}
func (self Enum) children() []SimpleType { return []SimpleType{} }
func (self TaggedUnion) children() []SimpleType {
	children := []SimpleType{}
	for _, variant := range self.Variants {
		if v, ok := variant.Unwrap(); ok {
			children = append(children, v)
		}
	}
	return children
}

// TODO: should probably at the very least return Application.Params
func (self Application) children() []SimpleType { return []SimpleType{} }

func (self Top) concrete()         {}
func (self Bot) concrete()         {}
func (self Func) concrete()        {}
func (self Record) concrete()      {}
func (self ObjectType) concrete()  {}
func (self ArrayType) concrete()   {}
func (self SliceType) concrete()   {}
func (self Primitive) concrete()   {}
func (self Int) concrete()         {}
func (self Str) concrete()         {}
func (self Ref) concrete()         {}
func (self Union) concrete()       {}
func (self Enum) concrete()        {}
func (self TaggedUnion) concrete() {}
func (self Application) concrete() {}

func (self Top) String() string { return "⊤" }
func (self Bot) String() string { return "⊥" }
func (self Func) String() string {
	return fmt.Sprintf("%s(%s -> %s)",
		If(self.Method, "#").Else(""),
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
	if self.Name != "" {
		return fmt.Sprintf("%s.%s", self.Module, self.Name)
	}
	return DeepPrint(self, false)
}
func (self ArrayType) String() string { return fmt.Sprintf("[%s %d]", self.ElType, self.Size) }
func (self SliceType) String() string { return fmt.Sprintf("[%s]", self.ElType) }
func (self Primitive) String() string { return string(self.Kind) }
func (self Int) String() string {
	if self.Signed {
		return "I" + strconv.FormatInt(self.BitSize, 10)
	} else {
		return "U" + strconv.FormatInt(self.BitSize, 10)
	}
}
func (self Str) String() string { return "Str" }
func (self Ref) String() string { return fmt.Sprintf("(Ref %s)", self.Content) }
func (self Union) String() string {
	variants := fun.Map(self.Variants, func(st ConcreteType) string {
		return st.String()
	})
	return fmt.Sprintf("(union %s {%s})", self.Name, strings.Join(variants, " "))
}
func (self TaggedUnion) String() string {
	var b strings.Builder
	b.WriteString("(tunion ")
	b.WriteString(self.Name)
	b.WriteString("{ ")
	for name, val := range self.Variants {
		b.WriteString("('")
		b.WriteString(name)
		b.WriteRune(' ')
		if v, ok := val.Unwrap(); ok {
			b.WriteString(v.String())
		}
		b.WriteString("), ")
	}
	b.WriteString("})")
	return b.String()
}
func (self Enum) String() string {
	var b strings.Builder
	b.WriteString("(enum ")
	b.WriteString(self.Name)
	b.WriteString("{ ")
	for name, val := range self.Values {
		b.WriteString(name)
		b.WriteRune(':')
		valStr := "?"
		if v, ok := val.Unwrap(); ok {
			valStr = strconv.Itoa(int(v))
		}
		b.WriteString(valStr)
		b.WriteByte(' ')
	}
	b.WriteString("})")
	return b.String()
}
func (self Application) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "(%s.%s", self.Module, self.Name)
	for _, param := range self.Params {
		b.WriteString(" ")
		b.WriteString(param.String())
	}
	b.WriteString(")")
	return b.String()
}

type deepPrintCtx struct {
	visited map[string]bool
	buf     strings.Builder

	recurseNamed bool
}

func DeepPrint(ty TypeScheme, recurseNamed bool) string {
	ctx := deepPrintCtx{
		visited:      map[string]bool{},
		recurseNamed: recurseNamed,
	}
	ctx.print(ty)
	return ctx.buf.String()
}

func (self *deepPrintCtx) print(ty TypeScheme) {
	switch t := ty.(type) {
	case Func:
		if t.Method {
			self.buf.WriteString("#")
		}
		for i, arg := range t.Args {
			if i != 0 {
				self.buf.WriteString(", ")
			}
			self.print(arg)
		}
		self.buf.WriteString(" -> ")
		self.print(t.Ret)
	case ObjectType:
		if !self.recurseNamed && t.Name != "" {
			self.buf.WriteString(t.String())
		} else {
			self.printObjectType(t)
		}
	case ArrayType:
	case PolymorphicType:
		self.buf.WriteString("polymorphic{")
		self.print(t.Body)
		self.buf.WriteString("}")
	// case Record:
	// case Ref:
	// case SliceType:
	// case Union:
	case *Variable:
		if t.representative != nil {
			fmt.Fprintf(&self.buf, "t%d=t%d[", t.uid, t.Representative().uid)
			self.print(t.LowerBound())
			self.buf.WriteString(", ")
			self.print(t.UpperBound())
			self.buf.WriteString("]")
			return
		}

		fmt.Fprintf(&self.buf, "t%d[", t.uid)
		self.print(t.LowerBound())
		self.buf.WriteString(", ")
		self.print(t.UpperBound())
		self.buf.WriteString("]")

	default:
		self.buf.WriteString(ty.String())
	}
}

func (self *deepPrintCtx) printObjectType(t ObjectType) {
	if t.Name == "" {
		self.buf.WriteString("_UnknownClass")
	} else {
		self.buf.WriteString(string(t.Module))
		self.buf.WriteString(".")
		self.buf.WriteString(t.Name)
	}

	if _, ok := self.visited[t.Name]; ok {
		self.buf.WriteString("{...}")
		return
	}
	self.visited[t.Name] = true

	self.buf.WriteString("{")
	for _, field := range t.Fields {
		self.buf.WriteString(field.Name)
		self.buf.WriteString(": ")
		self.print(field.Type)
		self.buf.WriteString(", ")
	}

	for _, meth := range t.Methods {
		self.buf.WriteString(meth.Name)
		self.buf.WriteString(": ")
		self.print(meth.Type)
		self.buf.WriteString(", ")
	}
	self.buf.WriteString("}")
}

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
		if len(work) > 100000 {
			panic("possible infinite loop")
		}
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

// helpers

type Member struct {
	Type   SimpleType
	Access parse.AccessLvl
}

type NamedMember struct {
	Name string
	Member
}

type NamedType struct {
	Name string
	Type SimpleType
}

type unit = struct{}

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

	// TODO: object types?
	if _, _, ok := matchPair[Top, Top](lhs, rhs); ok {
		return true
	} else if _, _, ok := matchPair[Bot, Bot](lhs, rhs); ok {
		return true
	} else if left, right, ok := matchPair[Primitive, Primitive](lhs, rhs); ok {
		return left.Kind == right.Kind
	} else if left, right, ok := matchPair[Int, Int](lhs, rhs); ok {
		return left.Signed == right.Signed && left.BitSize == right.BitSize
	} else if _, _, ok := matchPair[Str, Str](lhs, rhs); ok {
		return true
	} else if left, right, ok := matchPair[Ref, Ref](lhs, rhs); ok {
		return concreteEq_(left.Content, right.Content)
	} else if left, right, ok := matchPair[ArrayType, ArrayType](lhs, rhs); ok {
		return concreteEq_(left.ElType, right.ElType) && left.Size == right.Size
	} else if left, right, ok := matchPair[SliceType, SliceType](lhs, rhs); ok {
		return concreteEq_(left.ElType, right.ElType)
	} else if left, right, ok := matchPair[Union, Union](lhs, rhs); ok {
		return left.Name != right.Name
	} else if left, right, ok := matchPair[Enum, Enum](lhs, rhs); ok {
		return left.Name != right.Name
	} else if left, right, ok := matchPair[TaggedUnion, TaggedUnion](lhs, rhs); ok {
		if len(left.Variants) != len(right.Variants) {
			return false
		}
		for tag, lvar := range left.Variants {
			if rvar, ok := right.Variants[tag]; !ok {
				return false
			} else {
				if l, ok := lvar.Unwrap(); ok {
					return l == rvar.Or(nil)
				}
				return !rvar.IsSome()
			}
		}
		return true
	} else if left, right, ok := matchPair[Application, Application](lhs, rhs); ok {
		return left.Module == right.Module && left.Name == right.Name
	} else if left, right, ok := matchPair[ObjectType, ObjectType](lhs, rhs); ok {
		if left.Name != "" && left.Name == right.Name {
			return true
		}

		rightFields := namedMembersToMap(right.Fields)
		for _, field := range left.Fields {
			rightField, ok := rightFields[field.Name]
			if !ok {
				return false
			}

			if !concreteEq_(field.Type, rightField.Type) {
				return false
			}
		}

		rightMethods := namedMembersToMap(right.Methods)
		for _, meth := range left.Methods {
			rightMeth, ok := rightMethods[meth.Name]
			if !ok {
				return false
			}

			if !concreteEq_(meth.Type, rightMeth.Type) {
				return false
			}
		}
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

		for larg, rarg := range fun.ZipIterStrict(slices.Values(left.Args), slices.Values(right.Args)) {
			if !concreteEq_(larg, rarg) {
				return false
			}
		}

		return concreteEq_(left.Ret, right.Ret)
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

func sliceToSet[T comparable](xs []T) map[T]struct{} {
	s := map[T]struct{}{}
	for _, key := range xs {
		s[key] = struct{}{}
	}
	return s
}

func setToSlice[T comparable, V any](xs map[T]V) []T {
	s := make([]T, 0, len(xs))
	for k := range xs {
		s = append(s, k)
	}
	return s
}

var gIdCounter uint = 0

func newId() uint {
	gIdCounter++
	return gIdCounter
}

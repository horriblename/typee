package simplesub

import (
	"fmt"

	"github.com/horriblename/typee/src/fun"
	orderedset "github.com/horriblename/typee/src/internal/ordered_set"
	"github.com/horriblename/typee/src/opt"
	"github.com/horriblename/typee/src/types"
)

func SimplifyType(ty SimpleType) SimpleType {
	trace("simplifying: %s", ty)
	indentLvl++
	defer func() { indentLvl-- }()
	// TODO: idk if ordered set is needed, instead of unordered one
	pos := orderedset.NewOrderedSet[*Variable]()
	neg := orderedset.NewOrderedSet[*Variable]()

	analyze(ty, true, pos, neg)

	mapping := map[*Variable]SimpleType{}

	return transform(ty, true, mapping, pos, neg)
}

// Co-occurrence Analysis. Co-occurrence analysis looks at every variable that
// appears in a type in both positive and negative positions, and records along
// which other variables and types it always occurs. A variable 𝑣 occurs along
// a type 𝜏 if it is part of the same type union ... ⊔ 𝑣 ⊔ ... ⊔ 𝜏 ⊔ ... or
// part of the same type intersection ... ⊓ 𝑣 ⊓ ... ⊓ 𝜏 ⊓ ...
func analyze(st SimpleType, pol bool, pos, neg *orderedset.OrderedSet[*Variable]) {
	switch ty := st.(type) {
	case Record:
		for _, field := range ty.Fields {
			analyze(field.Type, pol, pos, neg)
		}
	case ObjectType:
		// TODO: fields
		for _, meth := range ty.Methods {
			analyze(meth.Type, pol, pos, neg)
		}
	case Func:
		for _, arg := range ty.Args {
			analyze(arg, !pol, pos, neg)
		}
		analyze(ty.Ret, pol, pos, neg)
	case *Variable:
		if pol {
			pos.Insert(ty)
			analyze(ty.LowerBound(), pol, pos, neg)
		} else {
			neg.Insert(ty)
			analyze(ty.UpperBound(), pol, pos, neg)
		}
	case Ref:
		analyze(ty.Content, pol, pos, neg)
	case Application:
		for _, param := range ty.Params {
			// FIXME: I'm probably using the wrong pol half the time,
			// but I'll need to wanna add variance semantics to type var annotations
			analyze(param, pol, pos, neg)
		}
	case Primitive, Int, Str, Top, Bot, Enum, Union: // Union bans generics
	}
}

func transformConcrete(st ConcreteType, pol bool, mapping map[*Variable]SimpleType, pos, neg *orderedset.OrderedSet[*Variable]) ConcreteType {
	switch ty := st.(type) {
	case Record:
		fields := fun.Map(ty.Fields, func(field NamedType) NamedType {
			return NamedType{field.Name, transform(field.Type, pol, mapping, pos, neg)}
		})

		return Record{fields}
	case ObjectType:
		fields := fun.Map(ty.Fields, func(field NamedMember) NamedMember {
			return NamedMember{field.Name, Member{transform(field.Type, pol, mapping, pos, neg), field.Access}}
		})
		methods := fun.Map(ty.Methods, func(meth NamedMember) NamedMember {
			return NamedMember{meth.Name, Member{transform(meth.Type, pol, mapping, pos, neg), meth.Access}}
		})
		return ObjectType{
			Name:    ty.Name,
			Supers:  ty.Supers,
			Fields:  fields,
			Methods: methods,
		}
	case Func:
		args := fun.Map(ty.Args, func(field SimpleType) SimpleType {
			return transform(field, !pol, mapping, pos, neg)
		})

		return Func{args, transform(ty.Ret, pol, mapping, pos, neg)}
	case ArrayType:
		return ArrayType{transform(ty.ElType, pol, mapping, pos, neg), ty.Size}
	case SliceType:
		return SliceType{transform(ty.ElType, pol, mapping, pos, neg)}
	case Ref:
		return Ref{transform(ty.Content, pol, mapping, pos, neg)}
	case Application:
		return Application{
			Module: ty.Module,
			Name:   ty.Name,
			Params: fun.Map(ty.Params, func(param SimpleType) SimpleType {
				return transform(param, pol, mapping, pos, neg)
			}),
		}
	case Primitive, Int, Str, Top, Bot, Union, Enum: // Union bans generics
		return st
	}
	panic("unreachable")
}

func transform(st SimpleType, pol bool, mapping map[*Variable]SimpleType, pos, neg *orderedset.OrderedSet[*Variable]) SimpleType {
	indentLvl++
	defer func() { indentLvl-- }()
	switch ty := st.(type) {
	case *Variable:
		if v, found := mapping[ty]; found {
			return v
		}

		if concreteEq(ty.LowerBound(), ty.UpperBound()) {
			trace("%s has same upper/lower bound, eliminating", ty)
			mapping[ty] = (transformConcrete(ty.LowerBound(), pol, mapping, pos, neg))
			return mapping[ty]
		} else if pol && !neg.Has(ty) {
			trace("%s is only in positive positions, eliminating", ty)
			// type variable only occurs on positive positions, we can eliminate it.
			// (see co-occurrence analysis)
			mapping[ty] = transformConcrete(ty.LowerBound(), pol, mapping, pos, neg)
			return mapping[ty]
		} else if !pol && !pos.Has(ty) {
			trace("%s is only in negative positions, eliminating", ty)
			mapping[ty] = transformConcrete(ty.UpperBound(), pol, mapping, pos, neg)
			return mapping[ty]
		} else {
			newVar := freshVar()
			newVar.lowerBound = transformConcrete(ty.LowerBound(), true, mapping, pos, neg)
			newVar.upperBound = transformConcrete(ty.UpperBound(), false, mapping, pos, neg)
			mapping[ty] = newVar
			return newVar
		}
	case ConcreteType:
		return transformConcrete(ty, pol, mapping, pos, neg)
	}
	panic(fmt.Sprintf("unhandled SimpleType %#v", st))
}

// Convert an inferred SimpleType into an immutable Type representation.
func CoalesceType(st SimpleType) types.Type {
	return coalesceTypeInner(st, true)
}

func coalesceTypeInner(st SimpleType, polarity bool) types.Type {
	switch ty := st.(type) {
	case *Variable:
		bound := If(polarity, ty.LowerBound()).Else(ty.UpperBound())
		boundTy := coalesceTypeInner(bound, polarity)
		if polarity && (bound == Bot{}) || (bound == Top{}) {
			return ty.asTypeVar()
		} else {
			if polarity {
				return &types.Join{Lhs: ty.asTypeVar(), Rhs: boundTy}
			} else {
				return &types.Inter{Lhs: ty.asTypeVar(), Rhs: boundTy}
			}
		}
	case Primitive:
		switch ty.Kind {
		case PrimitiveBool:
			return &types.Bool{}
		case PrimitiveFloat:
			return &types.Float{}
		case PrimitiveOpaque:
			return &types.Ref{}
		default:
			panic(fmt.Sprintf("unexpected simplesub.PrimitiveKind: %#v", ty.Kind))
		}
	case Int:
		return &types.Int{Signed: ty.Signed, BitSize: ty.BitSize}
	case Str:
		return &types.String{}
	case Ref:
		return &types.Ref{Content: opt.Some(coalesceTypeInner(ty.Content, polarity))}
	case Func:
		return &types.Func{
			Args: fun.Map(ty.Args, func(arg SimpleType) types.Type { return coalesceTypeInner(arg, !polarity) }),
			Ret:  coalesceTypeInner(ty.Ret, polarity),
		}
	case ArrayType:
		return &types.Array{
			Type: coalesceTypeInner(ty.ElType, polarity),
			Size: ty.Size,
		}
	case SliceType:
		return &types.Slice{Type: coalesceTypeInner(ty.ElType, polarity)}
	case Record:
		fields := map[string]types.Type{}
		for _, field := range ty.Fields {
			fields[field.Name] = coalesceTypeInner(field.Type, polarity)
		}
		return &types.Record{Fields: fields}
	case Union:
		set := orderedset.NewOrderedSet[types.Type]()
		for _, v := range ty.Variants {
			t := coalesceTypeInner(v, polarity)
			set.Insert(t)
		}

		return &types.Union{
			Name:     ty.Name,
			Variants: set,
		}
	case Enum:
		return &types.Enum{
			Name: ty.Name,
			Values: fun.MapMap(ty.Values, func(v opt.Option[int64]) int64 {
				if val, ok := v.Unwrap(); ok {
					return val
				}
				panic("TODO: coalesce unknown enum")
			}),
		}
	case ObjectType:
		fields := map[string]types.Member{}
		for _, field := range ty.Fields {
			fields[field.Name] = types.Member{
				Access: field.Access,
				Type:   coalesceTypeInner(field.Type, polarity),
			}
		}

		methods := map[string]types.Member{}
		for _, meth := range ty.Methods {
			methods[meth.Name] = types.Member{
				Access: meth.Access,
				Type:   coalesceTypeInner(meth.Type, polarity),
			}
		}

		supers := fun.Map(ty.Supers, func(o ObjectType) *types.Class {
			s := coalesceTypeInner(o, polarity)
			return s.(*types.Class)
		})

		return &types.Class{
			Name:    ty.Name,
			Supers:  supers,
			Fields:  fields,
			Statics: map[string]types.Member{},
			Methods: methods,
			Top:     ty.Top,
		}
	case Bot:
		return &types.Enum{Name: "", Values: map[string]int64{}}
	case Top:
		return &types.Top{}
	case Application:
		return &types.Application{
			Module: ty.Module,
			Name:   ty.Name,
			Params: fun.Map(ty.Params, func(param SimpleType) types.Type {
				return coalesceTypeInner(param, polarity)
			}),
		}

	default:
		panic(fmt.Sprintf("unexpected biunify.SimpleType: %#v", ty))
	}
}

type CursedIf[T any] struct {
	cond bool
	then T
}

func If[T any](cond bool, then T) CursedIf[T] {
	return CursedIf[T]{cond: cond, then: then}
}

func (self CursedIf[T]) Else(alt T) T {
	if self.cond {
		return self.then
	} else {
		return alt
	}
}

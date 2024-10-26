package biunify

import (
	"fmt"

	orderedset "github.com/horriblename/typee/src/biunify_simpler/internal/ordered_set"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/types"
)

func SimplifyType(ty SimpleType) SimpleType {
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
	case Func:
		for _, arg := range ty.Args {
			analyze(arg, pol, pos, neg)
		}
		analyze(ty.Ret, pol, pos, neg)
	case *Variable:
		if pol {
			pos.Insert(ty)
			analyze(ty.lowerBound, pol, pos, neg)
		} else {
			neg.Insert(ty)
			analyze(ty.upperBound, pol, pos, neg)
		}
	case Bool, Int, Str, Top, Bot:
	}
}

func transformConcrete(st ConcreteType, pol bool, mapping map[*Variable]SimpleType, pos, neg *orderedset.OrderedSet[*Variable]) ConcreteType {
	switch ty := st.(type) {
	case Record:
		fields := fun.Map(ty.Fields, func(field NamedType) NamedType {
			return NamedType{field.Name, transform(field.Type, pol, mapping, pos, neg)}
		})

		return Record{fields}
	case Func:
		args := fun.Map(ty.Args, func(field SimpleType) SimpleType {
			return transform(field, !pol, mapping, pos, neg)
		})

		return Func{args, transform(ty.Ret, pol, mapping, pos, neg)}
	case Bool, Int, Str, Top, Bot:
		return st
	}
	panic("unreachable")
}

func transform(st SimpleType, pol bool, mapping map[*Variable]SimpleType, pos, neg *orderedset.OrderedSet[*Variable]) SimpleType {
	switch ty := st.(type) {
	case *Variable:
		if v, found := mapping[ty]; found {
			return v
		}

		if concreteEq(ty.LowerBound(), ty.UpperBound()) {
			mapping[ty] = (transformConcrete(ty.lowerBound, pol, mapping, pos, neg))
			return mapping[ty]
		} else if pol && !neg.Has(ty) {
			// type variable only occurs on positive positions, we can eliminate it.
			// (see co-occurrence analysis)
			mapping[ty] = transformConcrete(ty.lowerBound, pol, mapping, pos, neg)
			return mapping[ty]
		} else if !pol && !pos.Has(ty) {
			mapping[ty] = transformConcrete(ty.upperBound, pol, mapping, pos, neg)
			return mapping[ty]
		} else {
			newVar := freshVar()
			newVar.lowerBound = transformConcrete(ty.lowerBound, true, mapping, pos, neg)
			newVar.upperBound = transformConcrete(ty.upperBound, false, mapping, pos, neg)
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
				return &types.Union{Lhs: ty.asTypeVar(), Rhs: boundTy}
			} else {
				return &types.Inter{Lhs: ty.asTypeVar(), Rhs: boundTy}
			}
		}
	case Bool:
		return &types.Bool{}
	case Int:
		return &types.Int{}
	case Str:
		return &types.String{}
	case Func:
		return &types.Func{
			Args: fun.Map(ty.Args, func(arg SimpleType) types.Type { return coalesceTypeInner(arg, !polarity) }),
			Ret:  coalesceTypeInner(ty.Ret, polarity),
		}
	case Record:
		fields := map[string]types.Type{}
		for _, field := range ty.Fields {
			fields[field.Name] = coalesceTypeInner(field.Type, polarity)
		}
		return &types.Record{Fields: fields}
	case Bot:
		return &types.Record{Fields: map[string]types.Type{}}
	case Top:
		return &types.Top{}
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

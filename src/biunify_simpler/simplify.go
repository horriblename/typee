package biunify

import (
	orderedset "github.com/horriblename/typee/src/biunify_simpler/internal/ordered_set"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/types"
)

func simplifyType(ty SimpleType) SimpleType {
	// TODO: idk if ordered set is needed, instead of unordered one
	pos := orderedset.NewOrderedSet[*Variable]()
	neg := orderedset.NewOrderedSet[*Variable]()

	analyze(ty, true, pos, neg)

	mapping := map[*Variable]SimpleType{}

	return transform(ty, true, mapping, pos, neg)
}

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
	case Bool, Top, Bot:
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
	case Bool, Top, Bot:
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

		if ty.lowerBound == ty.upperBound {
			mapping[ty] = (transformConcrete(ty.lowerBound, pol, mapping, pos, neg))
			return mapping[ty]
		} else if pol && !neg.Has(ty) {
			return transformConcrete(ty.lowerBound, pol, mapping, pos, neg)
		} else if !pol && !pos.Has(ty) {
			return transformConcrete(ty.lowerBound, pol, mapping, pos, neg)
		} else {
			return &Variable{
				lowerBound: transformConcrete(ty.lowerBound, true, mapping, pos, neg),
				upperBound: transformConcrete(ty.upperBound, false, mapping, pos, neg),
			}
		}
	case ConcreteType:
		return transformConcrete(ty, pol, mapping, pos, neg)
	}
	panic("unreachable")
}

// Convert an inferred SimpleType into an immutable Type representation.
func coalesceType(st SimpleType) {
	coalesceTypeInner(st, true)
}

func coalesceTypeInner(st SimpleType, polarity bool) types.Type {
	panic("unimpl")
}

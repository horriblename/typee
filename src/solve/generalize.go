package solve

import (
	"fmt"

	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/types"
)

// The job of generalize is to take a type like `'a -> 'a` and generalize it
// into a type scheme like `'a . 'a -> 'a` in an environment `env` against
// constraints `C1`.
//
// We need to first infer the let assignment's expression as if it were it's
// own program (genConstraints then unify then substitute type), then "apply
// substitutions also to env to get a new env1", idk what this means.
//
// After that we figure out which type in the type from above needs to be
// generalized, i.e. all the types generated from within this expression
//
// I squashed all of the above into this function (except env part) for
// convenience
func generalize(tt *TypeTable, cons *[]Constraint, node parse.Expr) (types.Type, []Constraint, error) {
	dbg("generalize let assignment: %v", node)
	typ, generics, err := genConstraints(tt, cons, node)
	if err != nil {
		return nil, nil, err
	}

	consCopy := make([]Constraint, 0, len(*cons))
	for _, c := range *cons {
		consCopy = append(consCopy, c.Clone())
	}

	subst, err := unify(*cons)
	if err != nil {
		return nil, nil, err
	}

	substituteAllToType(&typ, subst)

	// NOTE: the textbook approach is to walk the resulting typ and figure out
	// using env, which type variable is generated during this generalization.
	// It's not clear to me how to do this, so I opted to return a list of
	// generated type variables in [genConstraints] instead.

	// FIXME: I should apply substitutes to env???

	return &types.TypeScheme{Over: generics, Body: typ}, consCopy, nil
}

// instantiate takes a type scheme like `'a. 'a -> 'a` and instantiate it into
// a new type
func instantiate(tt *TypeTable, name string) (types.Type, error) {
	typ, ok := tt.ScopeStack.Find(name)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUndefinedVar, name)
	}

	typeScheme, ok := typ.(*types.TypeScheme)
	if !ok {
		return typ, nil
	}

	subs := make([]Subst, 0, len(typeScheme.Over))
	for _, v := range typeScheme.Over {
		subs = append(subs, Subst{
			Old: &v,
			New: types.NewGeneric("", "instantiated from "+v.String()),
		})
	}

	instance := types.Clone(typeScheme.Body)
	substituteAllToType(&instance, subs)

	return instance, nil
}

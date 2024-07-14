package solve

import (
	"fmt"
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/types"
)

func TestTypeInference(t *testing.T) {
	testCases := []struct {
		desc   string
		input  string
		expect types.Type
	}{
		{
			desc:   "Simple literal",
			input:  "1",
			expect: &types.Int{},
		},
		{
			desc:   "Simple if expr",
			input:  "(if [true] 1 2)",
			expect: &types.Int{},
		},
		// // Not entirely sure why this isn't possible
		// {
		// 	desc: "Nested if expr",
		// 	input: `
		// 		(if [(if [true] false true)]
		// 			(if [false]
		// 				1
		// 				(if [true] 2 3))
		// 			(if [false] 4 5))`,
		// 	expect: &types.Int{},
		// },
		{
			desc:  "Simple func def",
			input: "(def foo [x] 2)",
			expect: &types.Func{
				Args: []types.Type{&types.Generic{ID: 2}},
				Ret:  &types.Int{},
			},
		},
		{
			desc:  "func def + if",
			input: "(def foo [x] (if [x] 1 0))",
			expect: &types.Func{
				Args: []types.Type{&types.Bool{}},
				Ret:  &types.Int{},
			},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			println("---", tC.desc, "---")
			tassert := assert.NewTestAsserts(t)
			ast, err := parse.ParseString(tC.input)
			tassert.Ok(err, "parse failed")

			typ, cons, err := initConstraints(ast[0])
			tassert.Ok(err)

			t.Log("generated top-level type", typ)
			t.Log("constraints", cons)

			subs, err := unify(cons)
			tassert.Ok(err)

			t.Log("substitutions", subs)

			substituteAllToType(&typ, subs)
			fmt.Printf("resolved type: %v\n", typ)

			t.Log("typ: ", typ)
			t.Log("expect: ", tC.expect)
			tassert.True(types.StructuralEq(typ, tC.expect), "expected ", tC.expect, " got ", typ)
		})
	}
}

func TestGenConstraints(t *testing.T) {
	testCases := []struct {
		desc   string
		input  string
		typ    types.Type
		expect []Constraint
	}{
		{
			desc:   "Simple literal",
			input:  "false",
			typ:    &types.Bool{},
			expect: []Constraint{},
		},
		{
			desc:  "func def + if",
			input: "(def foo [x] (if [x] 0 1))",
			typ: &types.Func{
				Args: []types.Type{&types.Generic{ID: 1}},
				Ret:  &types.Generic{ID: 2},
			},
			expect: []Constraint{
				{&types.Generic{ID: 1}, &types.Bool{}},
				{&types.Generic{ID: 3}, &types.Int{}},
				{&types.Generic{ID: 3}, &types.Int{}},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			tassert := assert.NewTestAsserts(t)
			ast, err := parse.ParseString(tC.input)
			tassert.Ok(err, "parse failed")

			typ, cons, err := initConstraints(ast[0])
			tassert.Ok(err)

			tassert.True(types.StructuralEq(typ, tC.typ), "expected type", tC.typ, "got", typ)

			lhs := map_(cons, func(c Constraint) types.Type { return c.lhs })
			rhs := map_(cons, func(c Constraint) types.Type { return c.rhs })
			expectLhs := map_(tC.expect, func(c Constraint) types.Type { return c.lhs })
			expectRhs := map_(tC.expect, func(c Constraint) types.Type { return c.rhs })

			tassert.True(types.ListStructuralEq(lhs, expectLhs), "expected", tC.expect, "got", cons)
			tassert.True(types.ListStructuralEq(rhs, expectRhs), "expected", tC.expect, "got", cons)
		})
	}
}

func TestUnify(t *testing.T) {
	testCases := []struct {
		desc   string
		input  string
		typ    types.Type
		expect []Subst
	}{
		{
			desc:   "Simple literal",
			input:  "false",
			typ:    &types.Bool{},
			expect: []Subst{},
		},
		{
			desc:  "func def + if",
			input: "(def foo [x] (if [x] 0 1))",
			typ: &types.Func{
				Args: []types.Type{&types.Generic{ID: 1}},
				Ret:  &types.Generic{ID: 2},
			},
			expect: []Subst{
				{Old: &types.Generic{ID: 1}, New: &types.Bool{}},
				{Old: &types.Generic{ID: 3}, New: &types.Int{}},
				// third constraint discarded, as t3 is substituted with Int, hence: Int = Int
			},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			tassert := assert.NewTestAsserts(t)
			ast, err := parse.ParseString(tC.input)
			tassert.Ok(err, "parse failed")

			typ, cons, err := initConstraints(ast[0])
			tassert.Ok(err)

			tassert.True(types.StructuralEq(typ, tC.typ), "expected type", tC.typ, "got", typ)

			subs, err := unify(cons)
			tassert.Ok(err)

			old := map_(subs, func(s Subst) types.Type { return s.Old })
			new_ := map_(subs, func(s Subst) types.Type { return s.New })
			expectOld := map_(tC.expect, func(s Subst) types.Type { return s.Old })
			expectNew := map_(tC.expect, func(s Subst) types.Type { return s.New })

			tassert.True(types.ListStructuralEq(old, expectOld), "wrong substitution[...].Old\nexpected", tC.expect, "\ngot", subs)
			tassert.True(types.ListStructuralEq(new_, expectNew), "wrong substitution[...].New\nexpected", tC.expect, "\ngot", subs)
		})
	}
}

func map_[T, U any](xs []T, f func(T) U) []U {
	ys := make([]U, 0, len(xs))
	for _, x := range xs {
		ys = append(ys, f(x))
	}

	return ys
}

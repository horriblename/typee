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
			tassert.True(typ.Eq(tC.expect), "expected ", tC.expect, " got ", typ)
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
				{&types.Generic{ID: 2}, &types.Generic{ID: 3}},
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

			tassert.True(typ.Eq(tC.typ), "expected type", tC.typ, "got", typ)
			t.Logf("got %v\n expected %v", cons, tC.expect)
			if len(cons) != len(tC.expect) {
				t.Errorf("constraint set has different length than expected")
			}

			iMax := min(len(cons), len(tC.expect))
			for i := range iMax {
				if !cons[i].lhs.Eq(tC.expect[i].lhs) {
					t.Errorf("lhs of item %d is different", i)
				}
				if !cons[i].rhs.Eq(tC.expect[i].rhs) {
					t.Errorf("rhs of item %d is different", i)
				}
			}
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
				// third constraint discarded, as t#3 is substituted with Int, hence: Int = Int
				{Old: &types.Generic{ID: 2}, New: &types.Int{}},
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

			tassert.True(typ.Eq(tC.typ), "expected type", tC.typ, "got", typ)

			subs, err := unify(cons)
			tassert.Ok(err)

			t.Logf("got %v\n expected %v", subs, tC.expect)
			if len(subs) != len(tC.expect) {
				t.Errorf("substraint set has different length than expected")
			}

			iMax := min(len(subs), len(tC.expect))
			for i := range iMax {
				if !subs[i].Old.Eq(tC.expect[i].Old) {
					t.Errorf("lhs of item %d is different", i)
				}
				if !subs[i].New.Eq(tC.expect[i].New) {
					t.Errorf("rhs of item %d is different", i)
				}
			}

		})
	}
}

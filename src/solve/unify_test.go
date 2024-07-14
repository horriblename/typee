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
			input: "(def foo [x] (if [x] 1 0))",
			expect: &types.Func{
				Args: []types.Type{&types.Bool{}},
				Ret:  &types.Int{},
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

package solve

import (
	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/types"
	"testing"
)

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
			// note: while the exact number doesn't matter, equivalent Generics must
			// have the same ID
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
		{
			desc:  "multi-arg func def",
			input: "(def foo [x y] (if [x] y 4))",
			typ: &types.Func{
				Args: []types.Type{&types.Generic{ID: 1}, &types.Generic{ID: 2}},
				Ret:  &types.Generic{ID: 3},
			},
			// {} |- (def foo [x y] (if [x] y 4)) : t1, t2 -> t3 -| t1 = bool, t2 = t3, int = t3
			//	x: t1, y: t2 |- (if [x] y 4) : t3 -| t1 = bool, t2 = t3, int = t3
			//		x: t1, y: t2 |- x : t1 -| {}
			//		x: t1, y: t2 |- y : t2 -| {}
			//		x: t1, y: t2 |- 4 : int -| {}
			expect: []Constraint{
				{&types.Generic{ID: 1}, &types.Bool{}},
				{&types.Generic{ID: 3}, &types.Generic{ID: 2}},
				{&types.Generic{ID: 3}, &types.Int{}},
			},
		},
		{
			desc:  "func call",
			input: "(def foo [x] (+ x 2))",
			typ: &types.Func{
				Args: []types.Type{&types.Generic{ID: 1}},
				Ret:  &types.Generic{ID: 2},
			},
			expect: []Constraint{
				// mapping of generic variables: x: t1, (+ x 2): t2
				// {} -| (def foo [x] (+ x 2)) : t1 -> t2 |- (int, int -> int = t1, int -> t2)
				//   x: t1 -| (+ x 2) : t2 |- (int, int -> int = t1, int -> t2)
				//     x: t1 -| 2 : int
				{&intBinaryOpFuncType, &types.Func{
					Args: []types.Type{&types.Generic{ID: 1}, &types.Int{}},
					Ret:  &types.Generic{ID: 2},
				}},
			},
		},
		{
			desc:  "simple let in",
			input: "(let [id (fn [x] x)] (let [a (id 0)] (id true)))",
			typ:   t5,
			expect: []Constraint{
				{tyFn(t2, t2), tyFn(tyInt, t3)},
				{tyFn(t4, t4), tyFn(tyBool, t5)},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			tassert := assert.NewTestAsserts(t)
			ast, err := parse.ParseString(tC.input)
			tassert.Ok(err, "parse failed")

			typ, cons, err := initConstraints(&ScopeStack{}, ast[0])
			tassert.Ok(err)

			tassert.True(types.StructuralEq(typ, tC.typ), "expected type", tC.typ, "got", typ)

			lhs := map_(cons, func(c Constraint) types.Type { return c.lhs })
			rhs := map_(cons, func(c Constraint) types.Type { return c.rhs })
			expectLhs := map_(tC.expect, func(c Constraint) types.Type { return c.lhs })
			expectRhs := map_(tC.expect, func(c Constraint) types.Type { return c.rhs })

			tassert.True(types.ListStructuralEq(lhs, expectLhs), "\nexpected", (tC.expect), "\ngot", (cons))
			tassert.True(types.ListStructuralEq(rhs, expectRhs), "\nexpected", (tC.expect), "\ngot", (cons))
		})
	}
}

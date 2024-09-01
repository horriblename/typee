package solve

import (
	"fmt"
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/types"
)

// for readability
var t1 = &types.Generic{ID: 1}
var t2 = &types.Generic{ID: 2}
var t3 = &types.Generic{ID: 3}
var t4 = &types.Generic{ID: 4}
var t5 = &types.Generic{ID: 5}

var tyInt = &types.Int{}
var tyBool = &types.Bool{}

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
		{
			desc: "Nested if expr",
			input: `
				(if [(if [true] false true)]
					(if [false]
						1
						(if [true] 2 3))
					(if [false] 4 5))`,
			expect: &types.Int{},
		},
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
		{
			desc:  "func call",
			input: "(def foo [x] ((+ x) 2))",
			expect: &types.Func{
				Args: []types.Type{&types.Int{}},
				Ret:  &types.Int{},
			},
		},
		{
			desc:  "fn call",
			input: "(fn [x] (if [true] ((fn [y] (if [((> y) 3)] 4 ((* y) y))) 2) ((- 4) x)))",
			expect: &types.Func{
				Args: []types.Type{&types.Int{}},
				Ret:  &types.Int{},
			},
		},
		{
			desc:   "simple let expr",
			input:  "(let [id (fn [x] x)] (let [a (id 0)] (id true)))",
			expect: tyBool,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			println("---", tC.desc, "---")
			tassert := assert.NewTestAsserts(t)
			ast, err := parse.ParseString(tC.input)
			tassert.Ok(err, "parse failed")

			typ, err := Infer(ast[0])

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
			desc:  "func call",
			input: "(def foo [x] ((+ x) 2))",
			typ: &types.Func{
				Args: []types.Type{&types.Generic{ID: 1}},
				Ret:  &types.Generic{ID: 2},
			},
			expect: []Constraint{
				// mapping of generic variables: x: t1, (+ x): t2, ((+ x) 2): t3
				// {} -| (def foo [x] ((+ x) 2)) |- x: t1
				//   x: t1 -| ((+ x) 2) : t3
				//     x: t1 -| (+ x) : t2 |- {int -> int -> int = t1 -> t2}
				{&intBinaryOpFuncType, &types.Func{
					Args: []types.Type{&types.Generic{ID: 1}},
					Ret:  &types.Generic{ID: 2},
				}},
				//   x: t1 -| ((+ x) 2): t3 |- {t2 = int -> t3}
				{&types.Generic{ID: 2}, &types.Func{
					Args: []types.Type{&types.Int{}},
					Ret:  &types.Generic{ID: 3},
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

func prettifyConstraint(con Constraint, ctx *types.PrettyCtx) string {
	if ctx == nil {
		ctx = &types.PrettyCtx{}
	}
	return fmt.Sprintf("%v = %v", ctx.String(con.lhs), ctx.String(con.rhs))
}

func prettifyConstraints(cons []Constraint) []string {
	ctx := types.PrettyCtx{}
	return map_(cons, func(con Constraint) string { return prettifyConstraint(con, &ctx) })
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
		{
			desc:  "func call",
			input: "(def foo [x] ((+ x) 2))",
			typ: &types.Func{
				Args: []types.Type{&types.Generic{ID: 1}},
				Ret:  &types.Generic{ID: 2},
			},
			expect: []Subst{
				{Old: &types.Generic{ID: 1}, New: &types.Int{}},
				{Old: &types.Generic{ID: 2}, New: &types.Func{Args: []types.Type{&types.Int{}}, Ret: &types.Int{}}},
				{Old: &types.Generic{ID: 3}, New: &types.Int{}},
			},
		},
		{
			desc:  "simple let in",
			input: "(let [id (fn [x] x)] (let [a (id 0)] (id true)))",
			typ:   t4,
			expect: []Subst{
				{Old: t2, New: tyInt},
				{Old: t3, New: tyInt},
				{Old: t4, New: tyBool},
				{Old: t5, New: tyBool},
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
func tyFn(arg types.Type, ret types.Type) types.Type {
	return &types.Func{
		Args: []types.Type{arg},
		Ret:  ret,
	}
}

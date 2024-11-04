package simplesub

import (
	"errors"
	"slices"
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/types"
)

func TestTypeExpr(t *testing.T) {
	testCases := []struct {
		desc  string
		input string
		err   error
		typ   types.Type
	}{
		{
			desc:  "bool literal",
			input: "true",
			typ:   &types.Bool{},
		},
		{
			desc:  "int literal",
			input: "34",
			typ:   &types.Int{},
		},
		{
			desc:  "str literal",
			input: `"hi"`,
			typ:   &types.String{},
		},
		{
			desc:  "record literal",
			input: `{x: 1, y: true}`,
			typ: &types.Record{
				Fields: map[string]types.Type{
					"x": &types.Int{},
					"y": &types.Bool{},
				},
			},
		},
		{
			desc:  "if expr",
			input: "(if [true] 32 5)",
			typ:   &types.Int{},
		},
		{
			desc:  "if expr: different kind in branches",
			input: "(if [true] {} false)",
			err:   ErrIncompatibleTypes,
		},
		{
			desc:  "simple function",
			input: "(fn [x] 12)",
			typ: &types.Func{
				Args: []types.Type{&types.Generic{ID: 1}},
				Ret:  &types.Int{},
			},
		},
		{
			desc:  "application",
			input: "((fn [x] x) 34)",
			typ:   &types.Int{},
		},
		{
			desc:  "simple let expr",
			input: "(let [x 34 y {z: 20}] (if [true] x y.z))",
			typ:   &types.Int{},
		},
		// {
		// 	desc:  "local let expr does not generalize",
		// 	input: "(let [f (fn [x] x)] (let [y (f 3)] {f: f, y: y}))",
		// },
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			assert := assert.NewTestAsserts(t)
			checker := NewTyper(true)

			program, err := parse.ParseString(tC.input)
			assert.Ok(err)

			ty, err := checker.TypeTerm(program[0])
			assert.True(errors.Is(err, tC.err), "expected error", tC.err, ", got:", err)

			if tC.err != nil {
				return
			}
			t.Logf("pre-simplify: %v", ty)

			tySimp := SimplifyType(ty)
			t.Logf("simplified: %v", tySimp)

			typ := CoalesceType(tySimp)

			t.Logf("coalesced type: %v\n", typ)
			assert.NEq(tC.typ, nil, "bad test case")
			assert.True(types.StructuralEq(tC.typ, typ), "expected type", tC.typ, ", got:", typ)
		})
	}
}

func TestTypeProgram(t *testing.T) {
	testCases := []struct {
		desc  string
		input string
		typ   []types.Type
	}{
		{
			desc:  "basic function",
			input: "(def foo [x] x)",
			typ: []types.Type{
				&types.Func{
					Args: []types.Type{&types.Generic{ID: 1}},
					Ret:  &types.Generic{ID: 1},
				},
			},
		},
		{
			desc: "make sure instantiation works",
			input: `
					(def id [x] x)
					(set n (id 1))
					(set s (id "hi"))
				`,
			typ: []types.Type{
				&types.Func{
					Args: []types.Type{&types.Generic{ID: 1}},
					Ret:  &types.Generic{ID: 1},
				},
				&types.Int{},
				&types.String{},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			assert := assert.NewTestAsserts(t)
			checker := NewTyper(true)

			program, err := parse.ParseString(tC.input)
			assert.Ok(err)

			ty, err := checker.TypeProgram(program)
			assert.Ok(err)

			t.Logf("pre-simplify: %v", ty)

			tySimp := fun.Map(ty, func(ty PolymorphicType) SimpleType { return SimplifyType(ty.body) })
			t.Logf("simplified: %v", tySimp)

			typ := fun.Map(tySimp, CoalesceType)

			t.Logf("coalesced type: %v\n", typ)
			assert.NEq(tC.typ, nil, "bad test case")
			if len(tC.typ) != len(typ) {
				t.Errorf("expected %d results, got %d", len(tC.typ), len(typ))
			}
			for expect, got := range fun.ZipIter(slices.Values(tC.typ), slices.Values(typ)) {
				assert.True(types.StructuralEq(expect, got), "expected type", expect, ", got:", got)
			}
		})
	}
}

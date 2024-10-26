package biunify

import (
	"errors"
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/types"
)

func TestBiunify(t *testing.T) {
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
			desc:  "application",
			input: "((fn [x] x) 34)",
			typ:   &types.Int{},
		},
		{
			desc:  "simple let expr",
			input: "(let [x 34 y {z: 20}] (if [true] x y.z))",
			typ:   &types.Int{},
		},
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

			tySimp := simplifyType(ty)
			t.Logf("simplified: %v", tySimp)

			typ := coalesceType(tySimp)

			t.Logf("coalesced type: %v\n", typ)
			assert.NEq(tC.typ, nil, "bad test case")
			assert.True(types.StructuralEq(tC.typ, typ), "expected type", tC.typ, ", got:", typ)
		})
	}
}

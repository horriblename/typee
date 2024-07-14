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
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			tassert := assert.NewTestAsserts(t)
			ast, err := parse.ParseString(tC.input)
			tassert.Ok(err)

			typ, cons, err := initConstraints(ast[0])
			tassert.Ok(err)

			t.Log("constraints", cons)

			subs, err := unify(cons)
			tassert.Ok(err)

			t.Log("substitutions", subs)

			substituteAllToType(&typ, subs)
			fmt.Printf("resolved type: %v\n", typ)

			tassert.Eq(typ, tC.expect)
		})
	}
}

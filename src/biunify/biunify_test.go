package biunify

import (
	"fmt"
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/parse"
)

func Test(t *testing.T) {
	testCases := []struct {
		desc  string
		input string
	}{
		{
			desc:  "Simple bool",
			input: "true",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			assert := assert.NewTestAsserts(t)
			program, err := parse.ParseString(tC.input)
			assert.Ok(err)
			assert.Eq(len(program), 1)

			checker := TypeCheckerCore{}
			bindings := NewBindings()
			val, err := CheckExpr(&checker, bindings, program[0])
			assert.Ok(err)

			fmt.Printf("val: %v", val)
		})
	}
}

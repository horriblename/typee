package biunify

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/biunify/internal/reachable"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/parse"
)

func TestCheck(t *testing.T) {
	testCases := []struct {
		desc  string
		input string
		err   error
	}{
		{
			desc:  "Simple bool",
			input: "true",
		},
		{
			desc:  "Simple int",
			input: "42",
		},
		{
			desc:  "Simple string",
			input: `"hi"`,
		},
		{
			desc:  "If expr",
			input: "(let [x true] (if [x] false x))",
		},
		{
			desc:  "If expr different branch, literals",
			input: "(if [false] false {x: true})",
			err:   ErrIncompatibleKind,
		},
		{
			desc:  "I should make this fail",
			input: "(def foo [x] (x x))",
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
			assert.True(errors.Is(err, tC.err), "expected error:", tC.err, ", got", err)

			file, err := os.OpenFile("/tmp/graph.dot", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o655)
			assert.Ok(err)
			defer file.Close()
			labels := fun.Map(checker.types, func(t TypeNode) any { return fmt.Sprintf("%T", t) })

			err = reachable.ExportReachability(&checker.reachability, labels, file)
			assert.Ok(err)

			fmt.Printf("val: %v\n", val)
		})
	}
}

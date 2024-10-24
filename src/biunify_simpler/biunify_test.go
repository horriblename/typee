package biunify

import (
	"errors"
	"fmt"
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/parse"
)

func TestBiunify(t *testing.T) {
	testCases := []struct {
		desc  string
		input string
		err   error
		typ   TypeScheme
	}{
		{
			desc:  "bool literal",
			input: "true",
			typ:   Bool{},
		},
		{
			desc:  "int literal",
			input: "34",
			typ:   Int{},
		},
		{
			desc:  "str literal",
			input: `"hi"`,
			typ:   Str{},
		},
		{
			desc:  "if expr",
			input: "(if [true] 32 5)",
			typ:   Int{},
		},
		{
			desc:  "if expr: different kind in branches",
			input: "(if [true] {} false)",
			err:   ErrIncompatibleTypes,
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

			fmt.Printf("type: %#v\n", ty)
		})
	}
}

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
	}{
		{
			desc:  "bool literal",
			input: "true",
		},
		{
			desc:  "if expr",
			input: "(if [true] {} false)",
			err:   ErrCannotConstrain,
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

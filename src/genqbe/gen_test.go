package genqbe

import (
	"bytes"
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/solve"
)

func TestGen(t *testing.T) {
	testCases := []struct {
		desc   string
		input  string
		output string
	}{
		{
			desc:  "idk",
			input: "(def foo [x] (+ x 1))",
			output: //
			`function l $foo(l %x, l %y) {
	%c =l add %x, %y
	ret %c
}
`,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			assert := assert.NewTestAsserts(t)
			program, err := parse.ParseString(tC.input)
			assert.Ok(err)

			types, err := solve.Check(program)
			assert.Ok(err)

			var buf bytes.Buffer
			Gen(&buf, types, program)

			assert.Eq(buf.String(), tC.output)
		})
	}
}

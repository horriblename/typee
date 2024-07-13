package solve

import (
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/parse"
)

func TestTypeInference(t *testing.T) {
	testCases := []struct {
		desc  string
		input string
	}{
		{
			desc:  "",
			input: "1",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			ast, err := parse.ParseString(tC.input)
			assert.Ok(err)

			initConstraints(ast[0])
		})
	}
}

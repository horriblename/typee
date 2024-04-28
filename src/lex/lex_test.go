package lex

import (
	"reflect"
	"testing"
)

var lParen = LParen{}
var rParen = RParen{}
var tokSet = Set{}
var tokDef = Def{}

func TestLex(t *testing.T) {
	testCases := []struct {
		desc   string
		input  string
		output []Token
		err    error
	}{
		{
			desc:   "All",
			input:  "(foo)def set",
			output: []Token{&lParen, &Symbol{Name: "foo"}, &rParen, &tokDef, &tokSet},
			err:    nil,
		},
		{
			desc:   "keywords",
			input:  "def set defoo bar",
			output: []Token{&tokDef, &tokSet, &Symbol{Name: "defoo"}, &Symbol{Name: "bar"}},
		},
		{
			desc:   "simple form",
			input:  "(foo bar)",
			output: []Token{&lParen, &Symbol{Name: "foo"}, &Symbol{Name: "bar"}, &rParen},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			// Act
			got, gotErr := LexString(tC.input)

			if !reflect.DeepEqual(got, tC.output) {
				t.Errorf("expected output:\n  %v\ngot:\n  %v", tC.output, got)
			}

			if gotErr != tC.err {
				t.Errorf("expected error: %v, got: %v", tC.err, gotErr)
			}
		})
	}
}

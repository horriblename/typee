package lex

import (
	"reflect"
	"testing"
)

var lParen = LParen{}
var rParen = RParen{}

func TestLex(t *testing.T) {
	testCases := []struct {
		desc   string
		input  string
		output []Token
		err    error
	}{
		{
			desc:  "General",
			input: "(foo)",
			output: []Token{&lParen, &Symbol{
				String: "foo",
			}, &rParen},
			err: nil,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			// Act
			got, gotErr := LexString(tC.input)

			if !reflect.DeepEqual(got, tC.output) {
				t.Errorf("expected output:\n  %#v\ngot:\n  %#v", tC.output, got)
			}

			if gotErr == tC.err {
				t.Errorf("expected error: %v, got: %v", tC.output, got)
			}
		})
	}
}

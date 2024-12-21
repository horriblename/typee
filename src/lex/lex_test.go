package lex

import (
	"reflect"
	"testing"
)

var lParen = LParen{}
var rParen = RParen{}
var lBracket = LBracket{}
var rBracket = RBracket{}
var lBrace = LBrace{}
var rBrace = RBrace{}
var tokComma = Comma{}
var tokColon = Colon{}
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
			desc:  "All",
			input: `(foo)def[set, "str"]{123:.'label} var#::  r@if ;; this.is (a comment)`,
			output: []Token{&lParen, &Symbol{Name: "foo"}, &rParen, &tokDef,
				&lBracket, &tokSet, &tokComma, &StrLiteral{Content: "str"}, &rBracket,
				&lBrace, &IntLiteral{Number: 123}, &tokColon, &Dot{}, &Tag{Label: "label"}, &rBrace,
				&Var{}, &Hash{}, &DoubleColon{}, &Symbol{Name: "if", Raw: true},
			},
			err: nil,
		},
		{
			desc:  "Numbers",
			input: "13058 2.718282 -33 -12.3125",
			output: []Token{
				&IntLiteral{13058}, &FloatLiteral{2.718282}, &IntLiteral{-33}, &FloatLiteral{-12.3125},
			},
			err: nil,
		},
		{
			desc:  "keywords",
			input: "def set defoo bar true false if let fn case letrec class priv pub protected self interface new union enum type callExtern import",
			output: []Token{&tokDef, &tokSet, &Symbol{Name: "defoo"},
				&Symbol{Name: "bar"}, &TrueLiteral{}, &FalseLiteral{}, &If{},
				&Let{}, &Fn{}, &Case{}, &LetRec{}, &Class{}, &Priv{}, &Pub{},
				&Protected{}, &Self{}, &Interface{}, &New{}, &Union{}, &Enum{},
				&Type{}, &CallExtern{}, &Import{},
			},
		},
		{
			desc:   "simple form",
			input:  "(foo bar)",
			output: []Token{&lParen, &Symbol{Name: "foo"}, &Symbol{Name: "bar"}, &rParen},
		},
		{
			desc: "whitespace and comments",
			input: `

				(foo bar);hello


				(def id [x]

					x)

				; another comment
				; yet another comment

			`,
			output: []Token{&lParen, &Symbol{Name: "foo"}, &Symbol{Name: "bar"}, &rParen,
				&lParen, &Def{}, &Symbol{Name: "id"}, &lBracket, &Symbol{Name: "x"}, &rBracket,
				&Symbol{Name: "x"}, &rParen,
			},
			err: nil,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			// Act
			got, gotErr := LexString(tC.input)

			if gotErr != tC.err {
				t.Errorf("expected error: %v, got: %v", tC.err, gotErr)
			}

			if !reflect.DeepEqual(got, tC.output) {
				t.Errorf("expected output:\n  %v\ngot:\n  %v", tC.output, got)
			}
		})
	}
}

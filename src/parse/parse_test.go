package parse

import (
	"reflect"
	"testing"

	"github.com/horriblename/typee/src/opt"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		desc   string
		input  string
		output []Expr
	}{
		{
			desc:  "form",
			input: "(foo bar)",
			output: []Expr{&Form{
				children: []Expr{
					&Symbol{Name: "foo"},
					&Symbol{Name: "bar"},
				},
			}},
		},
		{
			desc:  "def",
			input: "(def foo (Str Int Str) [x y] (foo x y))",
			output: []Expr{&FuncDef{
				Name:      "foo",
				Signature: opt.Some([]string{"Str", "Int", "Str"}),
				Args:      []string{"x", "y"},
				Body: []Expr{&Form{
					children: []Expr{
						&Symbol{"foo"},
						&Symbol{"x"},
						&Symbol{"y"},
					},
				}},
			}},
		},
		{
			desc:  "def no function signature",
			input: "(def foo [x y] (foo x y))",
			output: []Expr{&FuncDef{
				Name:      "foo",
				Signature: opt.None[[]string](),
				Args:      []string{"x", "y"},
				Body: []Expr{&Form{
					children: []Expr{
						&Symbol{"foo"},
						&Symbol{"x"},
						&Symbol{"y"},
					},
				}},
			}},
		},
		{
			desc:  "set",
			input: "(set foo (+ x y))",
			output: []Expr{&Set{
				Name: "foo",
				rvalue: &Form{
					children: []Expr{
						&Symbol{"+"},
						&Symbol{"x"},
						&Symbol{"y"},
					},
				},
			}},
		},
		{
			desc:   "str literal",
			input:  `"strlit"`,
			output: []Expr{&StrLiteral{Content: "strlit"}},
		},
		{
			desc:   "int literal",
			input:  "123",
			output: []Expr{&IntLiteral{Number: 123}},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			got, err := parseString(tC.input)
			if err != nil {
				t.Logf("%#v", got)
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, tC.output) {
				t.Fatalf("expected output:\n  %#v\n  %#v", tC.output, got)
			}
		})
	}
}

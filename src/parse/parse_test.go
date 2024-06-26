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
				ID: 3,
				children: []Expr{
					&Symbol{ID: 1, Name: "foo"},
					&Symbol{ID: 2, Name: "bar"},
				},
			}},
		},
		{
			desc:  "def",
			input: "(def foo (Str Int Str) [x y] (foo x y))",
			output: []Expr{&FuncDef{
				ID:        5,
				Name:      "foo",
				Signature: opt.Some([]string{"Str", "Int", "Str"}),
				Args:      []string{"x", "y"},
				Body: []Expr{&Form{
					ID: 4,
					children: []Expr{
						&Symbol{ID: 1, Name: "foo"},
						&Symbol{ID: 2, Name: "x"},
						&Symbol{ID: 3, Name: "y"},
					},
				}},
			}},
		},
		{
			desc:  "def no function signature",
			input: "(def foo [x y] (foo x y))",
			output: []Expr{&FuncDef{
				ID:        5,
				Name:      "foo",
				Signature: opt.None[[]string](),
				Args:      []string{"x", "y"},
				Body: []Expr{&Form{
					ID: 4,
					children: []Expr{
						&Symbol{ID: 1, Name: "foo"},
						&Symbol{ID: 2, Name: "x"},
						&Symbol{ID: 3, Name: "y"},
					},
				}},
			}},
		},
		{
			desc:  "set",
			input: "(set foo (+ x y))",
			output: []Expr{&Set{
				ID:   5,
				Name: "foo",
				rvalue: &Form{
					ID: 4,
					children: []Expr{
						&Symbol{ID: 1, Name: "+"},
						&Symbol{ID: 2, Name: "x"},
						&Symbol{ID: 3, Name: "y"},
					},
				},
			}},
		},
		{
			desc:   "str literal",
			input:  `"strlit"`,
			output: []Expr{&StrLiteral{ID: 1, Content: "strlit"}},
		},
		{
			desc:   "int literal",
			input:  "123",
			output: []Expr{&IntLiteral{ID: 1, Number: 123}},
		},
		{
			desc:  "bool literal",
			input: "(foo true false)",
			output: []Expr{&Form{
				ID: 4,
				children: []Expr{
					&Symbol{ID: 1, Name: "foo"},
					&BoolLiteral{ID: 2, Value: true},
					&BoolLiteral{ID: 3, Value: false},
				},
			}},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			gIdCounter = 1
			got, err := parseString(tC.input)
			if err != nil {
				t.Logf("%#v", got)
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, tC.output) {
				t.Fatalf("expected output:\n  %+v\n  %+v", tC.output, got)
			}
		})
	}
}

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
				id: 3,
				children: []Expr{
					&Symbol{id: 1, Name: "foo"},
					&Symbol{id: 2, Name: "bar"},
				},
			}},
		},
		{
			desc:  "def",
			input: "(def foo (Str Int Str) [x y] (foo x y))",
			output: []Expr{&FuncDef{
				id:        5,
				Name:      "foo",
				Signature: opt.Some([]string{"Str", "Int", "Str"}),
				Args:      []string{"x", "y"},
				Body: []Expr{&Form{
					id: 4,
					children: []Expr{
						&Symbol{id: 1, Name: "foo"},
						&Symbol{id: 2, Name: "x"},
						&Symbol{id: 3, Name: "y"},
					},
				}},
			}},
		},
		{
			desc:  "def no function signature",
			input: "(def foo [x y] (foo x y))",
			output: []Expr{&FuncDef{
				id:        5,
				Name:      "foo",
				Signature: opt.None[[]string](),
				Args:      []string{"x", "y"},
				Body: []Expr{&Form{
					id: 4,
					children: []Expr{
						&Symbol{id: 1, Name: "foo"},
						&Symbol{id: 2, Name: "x"},
						&Symbol{id: 3, Name: "y"},
					},
				}},
			}},
		},
		{
			desc:  "set",
			input: "(set foo (+ x y))",
			output: []Expr{&Set{
				id:   5,
				Name: "foo",
				rvalue: &Form{
					id: 4,
					children: []Expr{
						&Symbol{id: 1, Name: "+"},
						&Symbol{id: 2, Name: "x"},
						&Symbol{id: 3, Name: "y"},
					},
				},
			}},
		},
		{
			desc:   "str literal",
			input:  `"strlit"`,
			output: []Expr{&StrLiteral{id: 1, Content: "strlit"}},
		},
		{
			desc:   "int literal",
			input:  "123",
			output: []Expr{&IntLiteral{id: 1, Number: 123}},
		},
		{
			desc:  "bool literal",
			input: "(foo true false)",
			output: []Expr{&Form{
				id: 4,
				children: []Expr{
					&Symbol{id: 1, Name: "foo"},
					&BoolLiteral{id: 2, Value: true},
					&BoolLiteral{id: 3, Value: false},
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

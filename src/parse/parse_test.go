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
				Children: []Expr{
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
					Children: []Expr{
						&Symbol{id: 1, Name: "foo"},
						&Symbol{id: 2, Name: "x"},
						&Symbol{id: 3, Name: "y"},
					},
				}},
			}},
		},
		{
			desc:  "def no function signature",
			input: "(def foo [x y] (foo x.bar y))",
			output: []Expr{&FuncDef{
				id:        5,
				Name:      "foo",
				Signature: opt.None[[]string](),
				Args:      []string{"x", "y"},
				Body: []Expr{&Form{
					id: 4,
					Children: []Expr{
						&Symbol{id: 1, Name: "foo"},
						&RecordAccess{id: 2, Record: "x", Field: "bar"},
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
				Value: &Form{
					id: 4,
					Children: []Expr{
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
			desc:  "if expr",
			input: "(if [true] (foo 1) 2)",
			output: []Expr{&IfExpr{
				id: 6,
				Condition: &BoolLiteral{
					id:    1,
					Value: true,
				},
				Consequence: &Form{
					id: 4,
					Children: []Expr{
						&Symbol{Name: "foo", id: 2},
						&IntLiteral{Number: 1, id: 3},
					},
				},
				Alternative: &IntLiteral{Number: 2, id: 5},
			}},
		},
		{
			desc:  "bool literal",
			input: "(foo true false)",
			output: []Expr{&Form{
				id: 4,
				Children: []Expr{
					&Symbol{id: 1, Name: "foo"},
					&BoolLiteral{id: 2, Value: true},
					&BoolLiteral{id: 3, Value: false},
				},
			}},
		},
		{
			desc:  "fn expression",
			input: "(fn [x] (+ x 1))",
			output: []Expr{&Fn{
				Arg: "x",
				Body: &Form{
					id: 4,
					Children: []Expr{
						&Symbol{id: 1, Name: "+"},
						&Symbol{id: 2, Name: "x"},
						&IntLiteral{id: 3, Number: 1},
					},
				},
			}},
		},
		{
			desc:  "let expr",
			input: "(let [x (+ 1 2) y (if [true] 3 4)] (* x y))",
			output: []Expr{&LetExpr{
				id: 13,
				Assignments: []Assignment{
					{Var: "x", Value: &Form{
						id: 4,
						Children: []Expr{
							&Symbol{id: 1, Name: "+"},
							&IntLiteral{id: 2, Number: 1},
							&IntLiteral{id: 3, Number: 2},
						},
					}},
					{Var: "y", Value: &IfExpr{
						id: 8,
						Condition: &BoolLiteral{
							id:    5,
							Value: true,
						},
						Consequence: &IntLiteral{id: 6, Number: 3},
						Alternative: &IntLiteral{id: 7, Number: 4},
					}},
				},
				Body: &Form{
					id: 12,
					Children: []Expr{
						&Symbol{id: 9, Name: "*"},
						&Symbol{id: 10, Name: "x"},
						&Symbol{id: 11, Name: "y"},
					},
				},
			}},
		},
		{
			desc:  "record",
			input: "{x: 12, y: (* 2 3)}",
			output: []Expr{&Record{
				id: 6,
				Fields: []RecordField{
					{Name: "x", Value: &IntLiteral{id: 1, Number: 12}},
					{Name: "y", Value: &Form{id: 5, Children: []Expr{
						&Symbol{id: 2, Name: "*"},
						&IntLiteral{id: 3, Number: 2},
						&IntLiteral{id: 4, Number: 3},
					}}},
				},
			}},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			gIdCounter = 1
			got, err := ParseString(tC.input)
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

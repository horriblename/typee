package parse

import (
	"reflect"
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/lex"
	"github.com/horriblename/typee/src/opt"
	"github.com/horriblename/typee/src/types"
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
				id:   5,
				Name: "foo",
				Signature: opt.Some([]TypeRepr{
					TypeName{"Str"},
					TypeName{"Int"},
					TypeName{"Str"},
				}),
				Args: []string{"x", "y"},
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
				id:        6,
				Name:      "foo",
				Signature: opt.None[[]TypeRepr](),
				Args:      []string{"x", "y"},
				Body: []Expr{&Form{
					id: 5,
					Children: []Expr{
						&Symbol{id: 1, Name: "foo"},
						&RecordAccess{id: 3, Record: &Symbol{"x", 2}, Field: "bar"},
						&Symbol{id: 4, Name: "y"},
					},
				}},
			}},
		},
		{
			desc:  "fn with signature",
			input: "(fn (Foo Bar) [x] x.bar)",
			output: []Expr{&Fn{
				Id: 3,
				Signature: opt.Some([]TypeRepr{
					TypeName{"Foo"},
					TypeName{"Bar"},
				}),
				Args: []string{"x"},
				Body: &RecordAccess{
					id:     2,
					Record: &Symbol{"x", 1},
					Field:  "bar",
				},
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
			desc:  "var definition",
			input: "(var foo 43)",
			output: []Expr{&VarDef{
				id:   2,
				Name: "foo",
				Value: &IntLiteral{
					id:     1,
					Number: 43,
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
			input: "(fn [x y] (+ x y))",
			output: []Expr{&Fn{
				Id:   5,
				Args: []string{"x", "y"},
				Body: &Form{
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
			desc:  "letrec expr",
			input: "(letrec [x 3] x)",
			output: []Expr{&LetExpr{
				id:        3,
				Recursive: true,
				Assignments: []Assignment{
					{Var: "x", Value: &IntLiteral{id: 1, Number: 3}},
				},
				Body: &Symbol{id: 2, Name: "x"},
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
		{
			desc:  "tagged expr",
			input: "('foo 42)",
			output: []Expr{&TaggedExpr{
				id:   2,
				Tag:  "foo",
				Body: &IntLiteral{id: 1, Number: 42},
			}},
		},
		{
			desc: "case expr",
			input: `(case x [
				('foo y) y
				('bar y) (+ y 1)
			])`,
			output: []Expr{&CaseExpr{
				id:    7,
				Match: &Symbol{id: 1, Name: "x"},
				Branches: []CaseBranch{
					{
						Pattern: CasePattern{Tag: "foo", Pattern: "y"},
						Body: &Symbol{
							id:   2,
							Name: "y",
						},
					},
					{
						Pattern: CasePattern{Tag: "bar", Pattern: "y"},
						Body: &Form{
							id: 6,
							Children: []Expr{
								&Symbol{id: 3, Name: "+"},
								&Symbol{id: 4, Name: "y"},
								&IntLiteral{Number: 1, id: 5},
							},
						},
					},
				},
			}},
		},
		{
			desc:  "empty class def",
			input: `(class Foo {})`,
			output: []Expr{&ClassDef{
				id:     1,
				Supers: []string{},
				Name:   "Foo",
				Fields: []ClassMember{},
			}},
		},
		{
			desc:  "class def",
			input: `(class Foo(Bar Baz) {pub foo Int,})`,
			output: []Expr{&ClassDef{
				id:     1,
				Name:   "Foo",
				Supers: []string{"Bar", "Baz"},
				Fields: []ClassMember{
					ClassField{
						Access_: types.AccessPublic,
						Name_:   "foo",
						Type:    TypeName{"Int"},
					},
				},
			}},
		},
		{
			desc:  "interface def",
			input: `(interface Foo {pub foo Int, protected (def foo [x] x)})`,
			output: []Expr{&InterfaceDef{
				id:     3,
				Name:   "Foo",
				Supers: []string{},
				Fields: []ClassMember{
					ClassField{
						Access_: types.AccessPublic,
						Name_:   "foo",
						Type:    TypeName{"Int"},
					},
					ClassMethod{
						Access_: types.AccessProtected,
						Func: &FuncDef{
							id:        2,
							Name:      "foo",
							Signature: opt.Option[[]TypeRepr]{},
							Args:      []string{"x"},
							Body: []Expr{&Symbol{
								id:   1,
								Name: "x",
							}},
						},
					},
				},
			}},
		},
		{
			desc:  "accessors",
			input: `(x#foo x.y)`,
			output: []Expr{&Form{
				id: 5,
				Children: []Expr{
					&MethodAccess{
						id:     2,
						Var:    &Symbol{"x", 1},
						Method: "foo",
					},
					&RecordAccess{
						id:     4,
						Record: &Symbol{"x", 3},
						Field:  "y",
					},
				},
			}},
		},
		{
			desc:  "constructor",
			input: `(Foo.new)`,
			output: []Expr{&Form{
				id: 3,
				Children: []Expr{
					&New{id: 2, Class: "Foo"},
				},
			}},
		},
		{
			desc:  "self",
			input: `(def meth (Self) [self] (self#meth self.x self))`,
			output: []Expr{&FuncDef{
				id:        7,
				Name:      "meth",
				Signature: opt.Some([]TypeRepr{SelfType{}}),
				Args:      []string{"self"},
				Body: []Expr{&Form{
					id: 6,
					Children: []Expr{
						&MethodAccess{
							id:     2,
							Var:    &SelfLiteral{1},
							Method: "meth",
						},
						&RecordAccess{
							id:     4,
							Record: &SelfLiteral{3},
							Field:  "x",
						},
						&SelfLiteral{5},
					},
				}},
			}},
		},
		{
			desc:  "enum definition",
			input: "(enum Foo {A B:34 C})",
			output: []Expr{&EnumDef{
				id:   1,
				Name: "Foo",
				Variants: []EnumVariant{
					{"A", opt.None[int64]()},
					{"B", opt.Some[int64](34)},
					{"C", opt.None[int64]()},
				},
			}},
		},
		{
			desc:  "enum access",
			input: "Foo::A",
			output: []Expr{&EnumAccess{
				id:   2,
				Enum: "Foo",
				Key:  "A",
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

func TestParseType(t *testing.T) {
	testCases := []struct {
		desc   string
		input  string
		output TypeRepr
	}{
		{
			desc:   "simple name",
			input:  "Foo",
			output: TypeName{"Foo"},
		},
		{
			desc:   "Self",
			input:  "Self",
			output: SelfType{},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			assert := assert.NewTestAsserts(t)
			tokens, err := lex.LexString(tC.input)
			assert.Ok(err)

			r1, got, err := type_(tokens)
			assert.Ok(err)
			assert.Eq(len(r1), 0)

			if !reflect.DeepEqual(got, tC.output) {
				t.Fatalf("expected output:\n  %+v\ngot:\n  %+v", tC.output, got)
			}
		})
	}
}

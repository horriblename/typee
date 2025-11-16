package parse

import (
	"reflect"
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/lex"
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
				id:   5,
				Name: "foo",
				Signature: opt.Some([]TypeRepr{
					TypeName{"", "Str"},
					TypeName{"", "Int"},
					TypeName{"", "Str"},
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
					TypeName{"", "Foo"},
					TypeName{"", "Bar"},
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
			desc:   "float literal",
			input:  "123.345",
			output: []Expr{&FloatLiteral{id: 1, Number: 123.345}},
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
			output: []Expr{&ObjectTypeDef{
				id:     1,
				Supers: []TypeName{},
				Name:   "Foo",
				Fields: []ClassMember{},
			}},
		},
		{
			desc:  "extern class def",
			input: `(class extern Foo {})`,
			output: []Expr{&ObjectTypeDef{
				id:     1,
				Supers: []TypeName{},
				Name:   "Foo",
				Fields: []ClassMember{},
				Extern: true,
			}},
		},
		{
			desc:  "type annotated method",
			input: `(def meth (Self Int Int) [self n] n)`,
			output: []Expr{&FuncDef{
				id:        2,
				Name:      "meth",
				Signature: opt.Some([]TypeRepr{SelfType{}, TypeName{"", "Int"}, TypeName{"", "Int"}}),
				Args:      []string{"self", "n"},
				Body:      []Expr{&Symbol{"n", 1}},
			}},
		},
		{
			desc:  "class def",
			input: `(class Foo(Bar Baz.Bar) {pub foo Int,})`,
			output: []Expr{&ObjectTypeDef{
				id:     1,
				Name:   "Foo",
				Supers: []TypeName{{"", "Bar"}, {"Baz", "Bar"}},
				Fields: []ClassMember{
					ClassField{
						Access_: AccessPublic,
						Name_:   "foo",
						Type:    TypeName{"", "Int"},
					},
				},
			}},
		},
		{
			desc:  "base class",
			input: `(class Foo({}) {pub foo Int,})`,
			output: []Expr{&ObjectTypeDef{
				id:     1,
				Name:   "Foo",
				Supers: []TypeName{},
				Fields: []ClassMember{
					ClassField{
						Access_: AccessPublic,
						Name_:   "foo",
						Type:    TypeName{"", "Int"},
					},
				},
				Base: true,
			}},
		},
		{
			desc:  "interface def",
			input: `(interface Foo {pub foo Int, protected (def foo [x] x)})`,
			output: []Expr{&ObjectTypeDef{
				id:     3,
				Kind:   Iface,
				Name:   "Foo",
				Supers: []TypeName{},
				Fields: []ClassMember{
					ClassField{
						Access_: AccessPublic,
						Name_:   "foo",
						Type:    TypeName{"", "Int"},
					},
					ClassMethod{
						Access_: AccessProtected,
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
						Obj:    &Symbol{"x", 1},
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
					&New{id: 2, Class: TypeName{"", "Foo"}},
				},
			}},
		},
		{
			desc:  "construct imported type",
			input: `(Foo.Bar.new)`,
			output: []Expr{&Form{
				id: 4,
				Children: []Expr{
					&New{id: 3, Class: TypeName{"Foo", "Bar"}},
				},
			}},
		},
		{
			desc:  "self",
			input: `(def meth (Self {}) [self] (self#meth self.x self))`,
			output: []Expr{&FuncDef{
				id:        7,
				Name:      "meth",
				Signature: opt.Some([]TypeRepr{SelfType{}, RecordType{Fields: []RecordTypeField{}}}),
				Args:      []string{"self"},
				Body: []Expr{&Form{
					id: 6,
					Children: []Expr{
						&MethodAccess{
							id:     2,
							Obj:    &SelfLiteral{1},
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
				Enum: TypeName{"", "Foo"},
				Key:  "A",
			}},
		},
		{
			desc:  "imported enum access",
			input: "Foo.Bar::A",
			output: []Expr{&EnumAccess{
				id:   3,
				Enum: TypeName{"Foo", "Bar"},
				Key:  "A",
			}},
		},
		{
			desc:  "union definition",
			input: "(union Foo {Int Str})",
			output: []Expr{&UnionDef{
				id:   1,
				Name: "Foo",
				Variants: []TypeRepr{
					TypeName{"", "Int"},
					TypeName{"", "Str"},
				},
			}},
		},
		{
			desc:  "type alias",
			input: "(type Foo Str)",
			output: []Expr{&TypeAlias{
				id:   1,
				Name: "Foo",
				Type: TypeName{"", "Str"},
			}},
		},
		{
			desc:  "extern call",
			input: "(callExtern exit 3)",
			output: []Expr{&ExternCall{
				id:     3,
				Symbol: Symbol{"exit", 2},
				Args: []Expr{
					&IntLiteral{3, 1},
				},
			}},
		},
		{
			desc:  "array literal",
			input: "[1 2 b c]",
			output: []Expr{&ArrayLiteral{
				id: 5,
				Elements: []Expr{
					&IntLiteral{1, 1},
					&IntLiteral{2, 2},
					&Symbol{"b", 3},
					&Symbol{"c", 4},
				},
			}},
		},
		{
			desc:  "import statement",
			input: "(import Foo.Bar)",
			output: []Expr{&Import{
				id:     1,
				Module: []string{"Foo", "Bar"},
			}},
		},
		{
			desc:  "extern declaration",
			input: "(extern def thing (Foo Str) [foo])",
			output: []Expr{&FuncDef{
				id:   1,
				Name: "thing",
				Signature: opt.Some([]TypeRepr{
					TypeName{"", "Foo"},
					TypeName{"", "Str"},
				}),
				Args:   []string{"foo"},
				Body:   []Expr{},
				Extern: true,
			}},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			gIdCounter = 1
			got, err := ParseString(tC.input)
			if err != nil {
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
			output: TypeName{"", "Foo"},
		},
		{
			desc:   "Self",
			input:  "Self",
			output: SelfType{},
		},
		{
			desc:  "record",
			input: "{a: Foo, b: {x: Int}}",
			output: RecordType{
				Fields: []RecordTypeField{
					{"a", TypeName{"", "Foo"}},
					{"b", RecordType{
						Fields: []RecordTypeField{
							{"x", TypeName{"", "Int"}},
						},
					}},
				},
			},
		},
		{
			desc:  "array type",
			input: "[[Foo 5]]",
			output: ArrayType{
				Type: ArrayType{
					Type: TypeName{"", "Foo"},
					Size: opt.Some(int64(5)),
				},
				Size: opt.None[int64](),
			},
		},
		{
			desc:  "type instantiation",
			input: "(Foo Int Str)",
			output: TypeInstantiation{
				Type: TypeName{"", "Foo"},
				Params: []TypeRepr{
					TypeName{"", "Int"},
					TypeName{"", "Str"},
				},
			},
		},
		{
			desc:  "fn type",
			input: "(fn [Int (Foo Int)] Bool)",
			output: FnType{
				Args: []TypeRepr{
					TypeName{"", "Int"},
					TypeInstantiation{
						Type:   TypeName{"", "Foo"},
						Params: []TypeRepr{TypeName{"", "Int"}},
					},
				},
				Ret: TypeName{"", "Bool"},
			},
		},
		{
			desc:   "imported type",
			input:  "GObject.Object",
			output: TypeName{"GObject", "Object"},
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
				t.Fatalf("expected output:\n  %#v\ngot:\n  %#v", tC.output, got)
			}
		})
	}
}

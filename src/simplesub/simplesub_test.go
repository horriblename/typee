package simplesub

import (
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/can"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/internal/ordered"
	orderedset "github.com/horriblename/typee/src/internal/ordered_set"
	"github.com/horriblename/typee/src/internal/scope"
	"github.com/horriblename/typee/src/opt"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/types"
)

var tI64 = types.Int{Signed: true, BitSize: 64}
var tI32 = types.Int{Signed: true, BitSize: 32}
var appI64 = types.Application{Name: "I64"}
var appObject = types.Application{
	Module: "Std",
	Name:   "Object",
	Params: []types.Type{},
}

const mainModule = "MainModule"

func tApp(name string, params ...types.Type) *types.Application {
	return &types.Application{
		Module: can.ModuleName(mainModule),
		Name:   name,
		Params: params,
	}
}

func ctorMember(name string) types.Member {
	return types.Member{
		Access: parse.AccessPublic,
		Type: &types.Func{
			Args:   []types.Type{},
			Ret:    tApp(name),
			Method: false,
		}}
}

func TestTypeExpr(t *testing.T) {
	testCases := []struct {
		desc  string
		input string
		err   error
		typ   types.Type
	}{
		{
			desc:  "bool literal",
			input: "true",
			typ:   &types.Bool{},
		},
		{
			desc:  "int literal",
			input: "34",
			typ:   &tI64,
		},
		{
			desc:  "float literal",
			input: "34.34",
			typ:   &types.Float{},
		},
		{
			desc:  "str literal",
			input: `"hi"`,
			typ:   &types.String{},
		},
		{
			desc:  "record literal",
			input: `{x: 1, y: true}`,
			typ: &types.Record{
				Fields: ordered.NewMap[string, types.Type]().
					With("x", &tI64).
					With("y", &types.Bool{}),
			},
		},
		{
			desc:  "if expr",
			input: "(if [true] 32 5)",
			typ:   &tI64,
		},
		{
			desc:  "if expr: different kind in branches",
			input: "(if [true] {} false)",
			err:   ErrIncompatibleTypes,
		},
		{
			desc:  "simple function",
			input: "(fn [x] 12)",
			typ: &types.Func{
				Args: []types.Type{&types.Top{}},
				Ret:  &tI64,
			},
		},
		{
			desc:  "type annotated function",
			input: "(fn (Int Int) [x] 12)",
			typ: &types.Func{
				Args: []types.Type{&tI64},
				Ret:  &tI64,
			},
		},
		{
			desc:  "type annotated function 2",
			input: "(fn ([Str] Str) [x] (at x 2))",
			typ: &types.Func{
				Args: []types.Type{
					&types.Slice{Type: &types.String{}},
				},
				Ret: &types.String{},
			},
		},
		{
			desc:  "application",
			input: "((fn [x] x) 34)",
			typ:   &tI64,
		},
		{
			desc:  "simple let expr",
			input: "(let [x 34 y 24] (if [true] x y))",
			typ:   &tI64,
		},
		{
			desc:  "local let expr does not generalize",
			input: "(let [f (fn [x] x)] (let [y (f 3)] {f: f, y: y}))",
			typ: &types.Record{
				Fields: ordered.NewMap[string, types.Type]().
					With("f", &types.Func{
						Args: []types.Type{&types.Generic{ID: 1}},
						Ret: &types.Join{
							Lhs: &types.Generic{ID: 1},
							Rhs: &tI64,
						},
					}).
					With("y", &tI64),
			},
		},
		{
			desc:  "list type",
			input: "(let [x 12] [1 2 x])",
			typ: &types.Slice{
				Type: &tI64,
			},
		},
		{
			desc:  "multi-assignment let expr",
			input: "(let [x 12 y 23 z false] (if [z] (+ x y) x))",
			typ:   &tI64,
		},
		{
			desc: "always allow Ref <: Opaque",
			input: `(let [
				x (stackAlloc)
				castIntRef (fn ((Ref Int) (Ref Int)) [y] y)
				f (fn (Opaque Int) [x] 5)
			] (f (castIntRef x)))`,
			typ: &tI64,
		},
		// // blocked by missing impl of constrain between different Application types
		// {
		// 	desc:  "type instantiation: type is not parameterized",
		// 	input: "(fn ((Int Str) Str) [x] x)",
		// 	err:   ErrUnparameterizedTypePassedParams,
		// },
		{
			desc:  "builtin types accessible via Std namespace",
			input: "(fn (Std.Object Int) [x] 0)",
			typ: &types.Func{
				Args: []types.Type{&types.Application{
					Module: "Std",
					Name:   "Object",
					Params: []types.Type{},
				}},
				Ret:    &tI64,
				Method: false,
			},
		},
		{
			desc:  "case expr",
			input: "(case (if [false] ('a 12) ('b false)) [('a x) (+ x 1) ('b x) 0])",
			err:   nil,
			typ:   &tI64,
		},
		{
			desc:  "case expr exhaustiveness is (technically) checked",
			input: "(case (if [false] ('a 12) ('b false)) [('a x) (+ x 1)])",
			err:   ErrTagMissing,
			typ:   nil,
		},
	}
	for _, tC := range testCases {
		EnableTrace = true
		t.Run(tC.desc, func(t *testing.T) {
			assert := assert.NewTestAsserts(t)
			checker := NewTyper(mainModule, true)

			program, err := parse.ParseString(tC.input)
			assert.Ok(err)

			ty, err := checker.TypeTerm(program[0])
			assert.True(errors.Is(err, tC.err), "expected error", tC.err, ", got:", err)

			if tC.err != nil {
				return
			}
			t.Logf("pre-simplify: %v", ty)

			tySimp := SimplifyType(ty)
			t.Logf("simplified: %v", tySimp)

			typ := CoalesceType(tySimp)

			t.Logf("coalesced type: %v\n", typ)
			assert.NEq(tC.typ, nil, "bad test case: err and typ are nil")
			assert.True(types.StructuralEq(tC.typ, typ), "expected type", tC.typ, ", got:", typ)
		})
	}
}

func TestTypeProgram(t *testing.T) {
	testCases := []struct {
		desc  string
		input string
		typ   []types.Type
	}{
		{
			desc:  "basic function",
			input: "(def foo [x] x)",
			typ: []types.Type{
				&types.Func{
					Args: []types.Type{&types.Generic{ID: 1}},
					Ret:  &types.Generic{ID: 1},
				},
			},
		},
		{
			desc: "make sure instantiation works",
			input: `
					(def id [x] x)
					(set n (id 1))
					(set s (id "hi"))
				`,
			typ: []types.Type{
				&types.Func{
					Args: []types.Type{&types.Generic{ID: 1}},
					Ret:  &types.Generic{ID: 1},
				},
				&tI64,
				&types.String{},
			},
		},
		{
			desc: "class definition",
			input: `
				(class Foo (Bar) {
					x Int,
					pub (def getx (Self Int) [self] self.x),
					pub (def print (Str {}) [name] (print name)),
				})
				(class Bar {y Str})
				(def foo (Foo {nothing: {}, foo: Foo}) [f]
					{foo: f, nothing: (Foo.print "hi")})
				(def main []
					(foo (Foo.new)))
			`,
			typ: func() []types.Type {
				bar := types.Class{
					Module: mainModule,
					Name:   "Bar",
					Supers: []*types.Application{&appObject},
					Fields: map[string]types.Member{
						"y": {
							Access: parse.AccessPrivate,
							Type:   &types.String{},
						},
					},
					Statics: map[string]types.Member{},
					Methods: ordered.NewMap[string, types.Member]().
						With("new", ctorMember("Bar")),
				}
				foo := types.Class{
					Name: "Foo",
					Supers: []*types.Application{{
						Module: mainModule,
						Name:   "Bar",
						Params: []types.Type{},
					}},
					Fields: map[string]types.Member{
						"x": {
							Access: parse.AccessPrivate,
							Type:   &tI64,
						},
					},
					Statics: map[string]types.Member{},
					Methods: ordered.NewMap[string, types.Member]().
						With("new", ctorMember("Foo")).
						With("getx", types.Member{
							Access: parse.AccessPublic,
							Type: &types.Func{
								Args:   []types.Type{},
								Ret:    &tI64,
								Method: true,
							},
						}).
						With("print", types.Member{
							Access: parse.AccessPublic,
							Type: &types.Func{
								Args: []types.Type{&types.String{}},
								Ret:  &types.Record{},
							},
						}),
					Module: mainModule,
					Top:    false,
				}

				return []types.Type{
					&foo,
					&bar,
					&types.Func{
						Args: []types.Type{tApp("Foo")},
						Ret: &types.Record{
							Fields: ordered.NewMap[string, types.Type]().
								With("nothing", &types.Record{}).
								With("foo", tApp("Foo")),
						},
					},
					&types.Func{
						Args: []types.Type{},
						Ret: &types.Record{
							Fields: ordered.NewMap[string, types.Type]().
								With("nothing", &types.Record{}).
								With("foo", tApp("Foo")),
						},
					},
				}
			}(),
		},
		{
			desc: "recursive class definition with super relation",
			input: `
				(class Baz (Bar) {})
				(class Foo {})
				(class Bar (Foo) {})
			`,
			typ: []types.Type{
				&types.Class{
					Module: mainModule,
					Name:   "Baz",
					Supers: []*types.Application{{
						Module: mainModule,
						Name:   "Bar",
						Params: []types.Type{},
					}},
					Fields:  map[string]types.Member{},
					Statics: map[string]types.Member{},
					Methods: ordered.NewMap[string, types.Member]().
						With("new", ctorMember("Baz")),
					Top: false,
				},
				&types.Class{
					Module:  mainModule,
					Name:    "Foo",
					Supers:  []*types.Application{&appObject},
					Fields:  map[string]types.Member{},
					Statics: map[string]types.Member{},
					Methods: ordered.NewMap[string, types.Member]().
						With("new", ctorMember("Foo")),
					Top: false,
				},
				&types.Class{
					Module: mainModule,
					Name:   "Bar",
					Supers: []*types.Application{{
						Module: mainModule,
						Name:   "Foo",
						Params: []types.Type{},
					}},
					Fields:  map[string]types.Member{},
					Statics: map[string]types.Member{},
					Methods: ordered.NewMap[string, types.Member]().
						With("new", ctorMember("Bar")),
					Top: false,
				},
			},
		},
		{
			desc:  "self and Self alias",
			input: "(class Foo {pub x Int, pub (def foo (Self Int) [self] self.x)})",
			typ: func() []types.Type {
				foo := &types.Class{
					Module: mainModule,
					Name:   "Foo",
					Supers: []*types.Application{&appObject},
					Fields: map[string]types.Member{
						"x": {
							Access: parse.AccessPublic,
							Type:   &tI64,
						},
					},
					Statics: map[string]types.Member{},
					Methods: ordered.NewMap[string, types.Member]().
						With("new", ctorMember("Foo")).
						With("foo", types.Member{
							Access: parse.AccessPublic,
							Type: &types.Func{
								Args:   []types.Type{},
								Ret:    &tI64,
								Method: true,
							},
						}),
				}
				return []types.Type{foo}
			}(),
		},
		{
			desc: "union",
			input: `
				(union Foo {Int Str})
				(def foo (Foo Int) [f] 32)
				(def testFoo [] (foo 42))
			`,
			typ: func() []types.Type {
				fooSet := orderedset.NewOrderedSet[types.Type](&types.String{})
				return []types.Type{
					&types.Union{
						Name:     "Foo",
						Variants: fooSet,
					},
					&types.Func{
						Args: []types.Type{tApp("Foo")},
						Ret:  &tI64,
					},
					&types.Func{
						Args: []types.Type{},
						Ret:  &tI64,
					},
				}
			}(),
		},
		{
			desc: "enum",
			input: `
				(enum Foo {A:1 B C})
				(def foo [] Foo::A)
			`,
			typ: func() []types.Type {
				foo := types.Enum{
					Name: "Foo",
					Values: map[string]int64{
						"A": 1,
						"B": 2,
						"C": 3,
					},
				}
				return []types.Type{
					&foo,
					&types.Func{
						Args: []types.Type{},
						Ret:  &foo,
					},
				}
			}(),
		},
		// https://github.com/horriblename/typee/issues/1
		// {
		// 	desc: "type alias",
		// 	input: `
		// 		(type Foo {x: Int})
		// 		(def x (Foo Foo) [foo] {x: (+ foo.x 1)})
		// 	`,
		// 	typ: []types.Type{
		// 		&types.Record{
		// 			Fields: map[string]types.Type{
		// 				"x": &tI64,
		// 			},
		// 		},
		// 		&types.Func{
		// 			Args: []types.Type{&types.Record{
		// 				Fields: map[string]types.Type{
		// 					"x": &tI64,
		// 				},
		// 			}},
		// 			Ret: &types.Record{
		// 				Fields: map[string]types.Type{
		// 					"x": &tI64,
		// 				},
		// 			},
		// 		},
		// 	},
		// },
		{
			desc: "record: mixed usage with object type",
			input: `
				(def checkX [foo] (if [(> foo.x 0)] foo foo))
				(def bar [] (let [
					foo (checkX {x: 3})
				]
					foo))
			`,
			typ: []types.Type{
				&types.Func{
					Args: []types.Type{
						&types.Inter{
							Lhs: &types.Generic{ID: 1},
							Rhs: &types.Class{
								Name:   "",
								Supers: []*types.Application{},
								Fields: map[string]types.Member{
									"x": {
										Access: parse.AccessPublic,
										Type:   &tI64,
									},
								},
								Statics: map[string]types.Member{},
								Methods: ordered.NewMap[string, types.Member](),
							},
						},
					},
					Ret: &types.Generic{ID: 1},
				},
				&types.Func{
					Args: []types.Type{},
					Ret: &types.Record{
						Fields: ordered.NewMap[string, types.Type]().
							With("x", &tI64),
					},
				},
			},
		},
		{
			desc:  "callExtern",
			input: "(def foo ({}) [] (callExtern exit 0))",
			typ: []types.Type{&types.Func{
				Args: []types.Type{},
				Ret: &types.Record{
					Fields: ordered.NewMap[string, types.Type](),
				}},
			},
		},
		// {
		// 	desc: "union return value",
		// 	input: `
		// 		(union Foo {Int Str})
		// 		(def bar (Foo) [] "hi")
		// 		(def testBar [] (bar))
		// 	`,
		// 	typ: func() []types.Type {
		// 		fooSet := orderedset.NewOrderedSet[types.Type](&types.String{})
		// 		fooUnion := types.Union{Name: "Foo", Variants: fooSet}
		// 		return []types.Type{
		// 			&types.Union{
		// 				Name:     "Foo",
		// 				Variants: fooSet,
		// 			},
		// 			&types.Func{
		// 				Args: []types.Type{},
		// 				Ret:  &fooUnion,
		// 			},
		// 			&types.Func{
		// 				Args: []types.Type{},
		// 				Ret:  &fooUnion,
		// 			},
		// 		}
		// 	}(),
		// },
		{
			desc: "callbacks",
			input: `
				(type Wrapper (fn [Int] {x: Int}))
				(def apply [f x] (f x))
				(def applySig (Wrapper Int {x: Int}) [f x] (f x))
				(def wrap [x] {x: x})
				(def foo [] (apply wrap 10))
				(def foo2 [] (applySig wrap 10))
			`,
			typ: func() []types.Type {
				wrappedInt := types.Record{
					Fields: ordered.NewMap[string, types.Type]().
						With("x", &tI64),
				}
				wrapper := types.Func{
					Args: []types.Type{&tI64},
					Ret:  &wrappedInt,
				}
				return []types.Type{
					&wrapper,
					&types.Func{
						Args: []types.Type{
							&types.Func{
								Args: []types.Type{&types.Generic{ID: 2}},
								Ret:  &types.Generic{ID: 3},
							},
							&types.Generic{ID: 2},
						},
						Ret: &types.Generic{ID: 3},
					},
					&types.Func{
						Args: []types.Type{tApp("Wrapper"), &tI64},
						Ret:  &wrappedInt,
					},
					&types.Func{
						Args: []types.Type{&types.Generic{ID: 1}},
						Ret: &types.Record{
							Fields: ordered.NewMap[string, types.Type]().
								With("x", &types.Generic{ID: 1}),
						},
					},
					&types.Func{
						Args: []types.Type{},
						Ret:  &wrappedInt,
					},
					&types.Func{
						Args: []types.Type{},
						Ret:  &wrappedInt,
					},
				}
			}(),
		},
		{
			desc: "let recursion",
			input: `
				(def foo [x] (bar (- x 2)))
				(def bar [x] (if [(< x 1)] 0 (foo (- x 1))))
			`,
			typ: []types.Type{
				&types.Func{
					Args: []types.Type{&tI64},
					Ret:  &tI64,
				},
				&types.Func{
					Args: []types.Type{&tI64},
					Ret:  &tI64,
				},
			},
		},
		{
			desc: "enum usage and type annotation",
			input: `
				(enum Grade {A B C})
				(def id [x] x)
				(def report (Grade Status {res: Int, grade: Grade, status: Status}) [grade status]
					{ res: (callExtern foo grade status)
					, grade: Grade::A
					, status: (id Status::Good)
					})
				(enum Status {Bad Good})
			`,
			typ: func() []types.Type {
				grade := &types.Enum{
					Name: "Grade",
					Values: map[string]int64{
						"A": 0,
						"B": 1,
						"C": 2,
					},
				}
				status := &types.Enum{
					Name: "Status",
					Values: map[string]int64{
						"Bad":  0,
						"Good": 1,
					},
				}
				return []types.Type{
					grade,
					&types.Func{
						Args: []types.Type{&types.Generic{ID: 2}},
						Ret:  &types.Generic{ID: 2},
					},
					&types.Func{
						Args: []types.Type{tApp("Grade"), tApp("Status")},
						Ret: &types.Record{
							Fields: ordered.NewMap[string, types.Type]().
								With("res", &tI64).
								With("grade", tApp("Grade")).
								With("status", tApp("Status")),
						},
					},
					status,
				}
			}(),
		},
		{
			desc: "import",
			input: `
				(import TestModule.Math)
				(def foo [x] (Math.addOne x))
			`,
			typ: []types.Type{
				&types.Func{
					Args: []types.Type{&tI64},
					Ret:  &tI64,
				},
			},
		},
		{
			desc: "imported super",
			input: `
				(import TestModule.Super)
				(class Foo (Super.Super) {})

				(def foo (Super.Super Int) [s] s.x)
				(def main [] (foo (Foo.new)))
			`,
			typ: []types.Type{
				&types.Class{
					Module: mainModule,
					Name:   "Foo",
					Supers: []*types.Application{
						{
							Module: "TestModule.Super",
							Name:   "Super",
							Params: []types.Type{},
						},
					},
					Methods: ordered.NewMap[string, types.Member]().
						With("new", ctorMember("Foo")),
					Top: false,
				},
				&types.Func{
					Args: []types.Type{&types.Application{
						Module: "TestModule.Super",
						Name:   "Super",
						Params: []types.Type{},
					}},
					Ret: &tI64,
				},
				&types.Func{
					Args: []types.Type{},
					Ret:  &tI64,
				},
			},
		},
		{
			desc:  "extern declaration",
			input: "(extern def doThing (Int Str) [n])",
			typ: []types.Type{
				&types.Func{
					Args: []types.Type{&tI64},
					Ret:  &types.String{},
				},
			},
		},
		{
			desc:  "type instantiation",
			input: "(def foo ((Ref Int)) [] (stackAlloc))",
			typ: []types.Type{
				&types.Func{
					Args: []types.Type{},
					Ret: &types.Ref{
						Content: opt.Some[types.Type](&tI64),
					},
				},
			},
		},
		{
			desc:  "ref types",
			input: "(def foo ((Ref Int)) [] (let [ptr (stackAlloc)] (let [x (+ (deref ptr) 1)] ptr)))",
			typ: []types.Type{
				&types.Func{
					Args: []types.Type{},
					Ret: &types.Ref{
						Content: opt.Some[types.Type](&tI64),
					},
				},
			},
		},
		{
			desc: "out of order type definitions still work",
			input: `
				(def f (Foo Bar) [foo] {y: 3})
				(type Foo {x: Bar})
				(type Bar {y: Int})
			`,
			typ: func() []types.Type {
				bar := types.Record{
					Fields: ordered.NewMap[string, types.Type]().
						With("y", &tI64),
				}
				foo := types.Record{
					Fields: ordered.NewMap[string, types.Type]().
						With("x", tApp("Bar")),
				}
				return []types.Type{
					&types.Func{
						Args: []types.Type{tApp("Foo")},
						Ret:  tApp("Bar"),
					},
					&foo,
					&bar,
				}
			}(),
		},
		{
			desc: "out of order class definition works",
			input: `
				(def testing (Foo Int) [foo] (foo#addOne))
				(class Foo {
					x I64,
					pub (def addOne (Self Int) [self] (+ self.x 1)),
				})
			`,
			typ: func() []types.Type {
				foo := types.Class{
					Module: mainModule,
					Name:   "Foo",
					Supers: []*types.Application{&appObject},
					Fields: map[string]types.Member{"x": {
						Access: parse.AccessPrivate, Type: &tI64}},
					Statics: map[string]types.Member{},
					Methods: ordered.NewMap[string, types.Member]().
						With("new", ctorMember("Foo")).
						With("addOne", types.Member{
							Access: parse.AccessPublic,
							Type: &types.Func{
								Args:   []types.Type{},
								Ret:    &tI64,
								Method: true,
							},
						}),
				}
				return []types.Type{
					&types.Func{
						Args: []types.Type{tApp("Foo")},
						Ret:  &tI64,
					},
					&foo,
				}
			}(),
		},
		{
			desc: "regression: infinite recursion when method calls function that takes Self",
			input: `
			(extern def g_binding_dup_source (Binding I32) [self_ ])
			(extern def g_binding_dup_target (Binding I32) [self_ ])
			(class Binding () {
				(def r@dupSource (Self I32) [self] (let [
					ret (g_binding_dup_source self)
				]
					ret)),
				(def r@dupTarget (Self I32) [self] (let [
					ret (g_binding_dup_target self)
				]
					ret))
			})
			`,
			typ: func() []types.Type {
				binding := types.Class{
					Module:  mainModule,
					Name:    "Binding",
					Supers:  []*types.Application{&appObject},
					Fields:  map[string]types.Member{},
					Statics: map[string]types.Member{},
					Methods: ordered.NewMap[string, types.Member]().
						With("new", ctorMember("Binding")).
						With("dupSource", types.Member{
							Access: 0,
							Type: &types.Func{
								Args:   []types.Type{},
								Ret:    &tI32,
								Method: true,
							},
						}).
						With("dupTarget", types.Member{
							Access: 0,
							Type: &types.Func{
								Args:   []types.Type{},
								Ret:    &tI32,
								Method: true,
							},
						}),
					Top: false,
				}
				return []types.Type{
					&types.Func{Args: []types.Type{tApp("Binding")}, Ret: &tI32},
					&types.Func{Args: []types.Type{tApp("Binding")}, Ret: &tI32},
					&binding,
				}
			}(),
		},
		{
			desc: "mutually recursive class definition",
			input: `
				(class Foo {
					x I64,
					pub (def add (Self Bar Int) [self bar] 1),
				})
				(class Bar {
					y I64,
					pub (def add (Self Foo Int) [self foo] (+ self.y foo.x)),
				})
			`,
			typ: []types.Type{
				&types.Class{
					Module: mainModule,
					Name:   "Foo",
					Supers: []*types.Application{&appObject},
					Fields: map[string]types.Member{
						"x": {
							Type:   &tI64,
							Access: parse.AccessPrivate,
						}},
					Methods: ordered.NewMap[string, types.Member]().
						With("new", ctorMember("Foo")).
						With("add", types.Member{
							Access: parse.AccessPublic,
							Type: &types.Func{
								Args:   []types.Type{tApp("Bar")},
								Ret:    &tI64,
								Method: true,
							},
						}),
					Top:     false,
					Statics: map[string]types.Member{},
				},
				&types.Class{
					Module: mainModule,
					Name:   "Bar",
					Supers: []*types.Application{&appObject},
					Fields: map[string]types.Member{
						"y": {
							Access: parse.AccessPrivate,
							Type:   &tI64,
						},
					},
					Statics: map[string]types.Member{},
					Methods: ordered.NewMap[string, types.Member]().
						With("new", ctorMember("Bar")).
						With("add", types.Member{
							Access: parse.AccessPublic,
							Type: &types.Func{
								Args:   []types.Type{tApp("Foo")},
								Ret:    &tI64,
								Method: true,
							},
						}),
					Top: false,
				},
			},
		},
		{
			desc: "regression: constraining type var of object type should not fail occursCheck",
			input: `
				(class Foo {
					x Int,
					pub (def hello (Self {}) [self] (print "Hello"))
				})

				(def main [] (let [foo (Foo.new)] (foo#hello)))
			`,
			typ: []types.Type{
				&types.Class{
					Module: mainModule,
					Name:   "Foo",
					Supers: []*types.Application{&appObject},
					Fields: map[string]types.Member{
						"x": {
							Access: parse.AccessPrivate,
							Type:   &tI64,
						},
					},
					Statics: map[string]types.Member{},
					Methods: ordered.NewMap[string, types.Member]().
						With("new", ctorMember("Foo")).
						With("hello", types.Member{
							Access: parse.AccessPublic,
							Type: &types.Func{
								Args:   []types.Type{},
								Ret:    &types.Record{},
								Method: true,
							},
						}),
					Top: false,
				},
				&types.Func{
					Args: []types.Type{},
					Ret:  &types.Record{},
				},
			},
		},
		{
			desc: "Std alias works in imported module",
			input: `
				(import TestModule.ImportedStdAlias)
				(def doNothing (ImportedStdAlias.Int Int) [x] 0)
				(def main [] (doNothing 5))
			`,
			typ: []types.Type{
				&types.Func{
					Args: []types.Type{
						&types.Application{
							Module: "TestModule.ImportedStdAlias",
							Name:   "Int",
							Params: []types.Type{},
						},
					},
					Ret:    &tI64,
					Method: false,
				},
				&types.Func{
					Args:   []types.Type{},
					Ret:    &tI64,
					Method: false,
				},
			},
		},
		{
			desc: "class up-casting",
			input: `
				(class Animal {
					pub name Str,
				})
				(class Mammal (Animal) {})
				(class Cat (Mammal) {})

				(def animalName (Animal Str) [a] a.name)
				(def test [] (print (animalName (Cat.new))))
			`,
			typ: []types.Type{
				&types.Class{
					Module: mainModule,
					Kind:   parse.Class,
					Name:   "Animal",
					Supers: []*types.Application{&appObject},
					Fields: map[string]types.Member{
						"name": {
							Access: parse.AccessPublic,
							Type:   &types.String{},
						},
					},
					Statics: map[string]types.Member{},
					Methods: ordered.NewMap[string, types.Member]().
						With("new", ctorMember("Animal")),
					Top: false,
				},
				&types.Class{
					Module:  mainModule,
					Kind:    parse.Class,
					Name:    "Mammal",
					Supers:  []*types.Application{tApp("Animal")},
					Fields:  map[string]types.Member{},
					Statics: map[string]types.Member{},
					Methods: ordered.NewMap[string, types.Member]().
						With("new", ctorMember("Mammal")),
					Top: false,
				},
				&types.Class{
					Module:  mainModule,
					Kind:    parse.Class,
					Name:    "Cat",
					Supers:  []*types.Application{tApp("Mammal")},
					Fields:  map[string]types.Member{},
					Statics: map[string]types.Member{},
					Methods: ordered.NewMap[string, types.Member]().
						With("new", ctorMember("Cat")),
					Top: false,
				},
				&types.Func{
					Args: []types.Type{
						tApp("Animal"),
					},
					Ret:    &types.String{},
					Method: false,
				},
				&types.Func{
					Args:   []types.Type{},
					Ret:    &types.Record{},
					Method: false,
				},
			},
		},
		{
			desc: "call of superclass method",
			input: `
				(class Foo {
					pub (def name (Self Str) [self] "foo")
				})
				(class Bar (Foo) {})
				(def main [] (let [
					bar (Bar.new)
				] (print (bar#name))))
			`,
			typ: []types.Type{
				&types.Class{
					Module:  mainModule,
					Kind:    parse.Class,
					Name:    "Foo",
					Supers:  []*types.Application{&appObject},
					Fields:  map[string]types.Member{},
					Statics: map[string]types.Member{},
					Methods: ordered.NewMap[string, types.Member]().
						With("new", types.Member{
							Access: parse.AccessPublic,
							Type: &types.Func{
								Args:   []types.Type{},
								Ret:    tApp("Foo"),
								Method: false,
							},
						}).
						With("name", types.Member{
							Access: parse.AccessPublic,
							Type: &types.Func{
								Args:   []types.Type{},
								Ret:    &types.String{},
								Method: true,
							},
						}),
					Top: false,
				},
				&types.Class{
					Module:  mainModule,
					Kind:    parse.Class,
					Name:    "Bar",
					Supers:  []*types.Application{tApp("Foo")},
					Fields:  map[string]types.Member{},
					Statics: map[string]types.Member{},
					Methods: ordered.NewMap[string, types.Member]().
						With("new", types.Member{
							Access: parse.AccessPublic,
							Type: &types.Func{
								Args:   []types.Type{},
								Ret:    tApp("Bar"),
								Method: false,
							},
						}),
					Top: false,
				},
				&types.Func{
					Args:   []types.Type{},
					Ret:    &types.Record{},
					Method: false,
				},
			},
		},
		{
			desc: "case expression",
			input: `
				(def f [tu] (case tu [
					('foo y) ('c (print (i64ToStr y)))
					('bar y) ('a (+ y 1))
					('baz s) ('c (print s))
				]))
				(def main []
					(let [
						_ (f ('bar 23))
					]
						0))
			`,
			typ: []types.Type{
				&types.Func{
					Args: []types.Type{&types.TaggedUnion{
						Name: "",
						Variants: ordered.NewMap[string, opt.Option[types.Type]]().
							With("bar", opt.Some[types.Type](&tI64)).
							With("baz", opt.Some[types.Type](&types.String{})).
							With("foo", opt.Some[types.Type](&tI64)),
					}},
					Ret: &types.TaggedUnion{
						Name: "",
						Variants: ordered.NewMap[string, opt.Option[types.Type]]().
							With("a", opt.Some[types.Type](&tI64)).
							With("c", opt.Some[types.Type](&types.Record{})),
					},
					Method: false,
				},
				&types.Func{
					Args:   []types.Type{},
					Ret:    &tI64,
					Method: false,
				},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			EnableTrace = true
			assert := assert.NewTestAsserts(t)
			checker := NewTyper(mainModule, true)

			program, err := parse.ParseString(tC.input)
			assert.Ok(err, "parse error")

			ty, _, err := checker.TypeProgram(program)
			assert.Ok(err, "type error")

			t.Logf("pre-simplify: %v", ty)

			tySimp := fun.Map(ty, func(ts TypeScheme) SimpleType {
				if pt, ok := ts.(PolymorphicType); ok {
					return SimplifyType(pt.instantiate())
				} else {
					return SimplifyType(ts.(SimpleType))
				}
			})
			t.Logf("simplified: %v", tySimp)

			typ := fun.Map(tySimp, CoalesceType)

			t.Logf("coalesced type: %v\n", fun.Map(typ, types.DeepPrint))
			assert.NEq(tC.typ, nil, "bad test case")
			if len(tC.typ) != len(typ) {
				t.Errorf("expected %d results, got %d", len(tC.typ), len(typ))
			}
			for expect, got := range fun.ZipIterStrict(slices.Values(tC.typ), slices.Values(typ)) {
				if !types.StructuralEq(expect, got) {
					t.Errorf("expected type\n  %v\ngot:\n  %v", types.DeepPrint(expect), types.DeepPrint(got))
				}
			}
		})
	}
}

func TestClosureCapture(t *testing.T) {
	testCases := []struct {
		desc     string
		input    string
		captures []map[string]unit
	}{
		{
			desc: "",
			input: `
				(def foo [x]
					(let [
						y 10
						f (fn [o]
							(- (+ x y) o))
						g (fn [o]
							((fn []
								(- (+ o y) 10))))
					]
						(fn [] f)))
			`,
			captures: []map[string]unit{
				newSet("x", "y"),
				newSet("y", "o"),
				newSet("y"),
				newSet("f"),
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			assert := assert.NewTestAsserts(t)
			ast, err := parse.ParseString(tC.input)
			assert.Ok(err, "parse error")

			ctx := findClosureCtx{
				vars:               scope.NewScopedMap[int](),
				outOfScopeAccesses: []outsideAccesses{},
				innerMostFnLevel:   0,
				Captures:           map[int][]Capture{},
			}

			for _, expr := range ast {
				captureClosures(&ctx, expr)
			}

			astIds := slices.Sorted(maps.Keys(ctx.Captures))
			got := fun.Map(astIds, func(id int) map[string]unit {
				return sliceToSet(fun.Map(ctx.Captures[id], func(c Capture) string {
					return c.Name
				}))
			})

			assert.DeepEq(tC.captures, got)
		})
	}
}

func newSet[T comparable](xs ...T) map[T]unit {
	s := map[T]unit{}
	for _, el := range xs {
		s[el] = unit{}
	}
	return s
}

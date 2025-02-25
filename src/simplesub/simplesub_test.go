package simplesub

import (
	"errors"
	"slices"
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/fun"
	orderedset "github.com/horriblename/typee/src/internal/ordered_set"
	"github.com/horriblename/typee/src/opt"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/types"
)

var tI64 = types.Int{Signed: true, BitSize: 64}

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
				Fields: map[string]types.Type{
					"x": &tI64,
					"y": &types.Bool{},
				},
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
				Fields: map[string]types.Type{
					"f": &types.Func{
						Args: []types.Type{&types.Generic{ID: 1}},
						Ret: &types.Join{
							Lhs: &types.Generic{ID: 1},
							Rhs: &tI64,
						},
					},
					"y": &tI64,
				},
			},
		},
		{
			desc:  "array type",
			input: "(let [x 12] [1 2 x])",
			typ: &types.Array{
				Type: &tI64,
				Size: 3,
			},
		},
		{
			desc:  "multi-assignment let expr",
			input: "(let [x 12 y 23 z false] (if [z] (+ x y) x))",
			typ:   &tI64,
		},
		{
			desc:  "type instantiation: type is not parameterized",
			input: "(fn ((Int Str) Str) [x] x)",
			err:   ErrUnparameterizedTypePassedParams,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			assert := assert.NewTestAsserts(t)
			checker := NewTyper("MainModule", true)

			program, err := parse.ParseString(tC.input)
			assert.Ok(err)

			ty, err := checker.TypeTerm(&moduleContext{map[int]TypeScheme{}, map[string]ModuleInfo{}}, program[0])
			assert.True(errors.Is(err, tC.err), "expected error", tC.err, ", got:", err)

			if tC.err != nil {
				return
			}
			t.Logf("pre-simplify: %v", ty)

			tySimp := SimplifyType(ty)
			t.Logf("simplified: %v", tySimp)

			typ := CoalesceType(tySimp)

			t.Logf("coalesced type: %v\n", typ)
			assert.NEq(tC.typ, nil, "bad test case")
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
				(class Foo (Bar) {x Int, pub (def getx (Self Int) [self] self.x)})
				(class Bar {y Str})
				(def foo (Foo Foo) [f] f)
				(def main []
					(foo (Foo.new)))
			`,
			typ: func() []types.Type {
				bar := types.Class{
					Name:   "Bar",
					Supers: []*types.Class{},
					Fields: map[string]types.Member{
						"y": {
							Access: types.AccessPrivate,
							Type:   &types.String{},
						},
					},
					Statics: map[string]types.Member{},
					Methods: map[string]types.Member{},
				}
				foo := types.Class{
					Name:   "Foo",
					Supers: []*types.Class{&bar},
					Fields: map[string]types.Member{
						"x": {
							Access: types.AccessPrivate,
							Type:   &tI64,
						},
					},
					Statics: map[string]types.Member{},
					Methods: map[string]types.Member{
						"getx": {
							Access: types.AccessPublic,
							Type: &types.Func{
								Args: []types.Type{&types.Class{Name: "Foo"}},
								Ret:  &tI64,
							},
						},
					},
				}

				return []types.Type{
					&foo,
					&bar,
					&types.Func{
						Args: []types.Type{&foo},
						Ret:  &foo,
					},
					&types.Func{
						Args: []types.Type{},
						Ret:  &foo,
					},
				}
			}(),
		},
		{
			desc:  "self and Self alias",
			input: "(class Foo {pub x Int, pub (def foo  [self] self.x)})",
			typ: func() []types.Type {
				foo := types.Class{}
				foo = types.Class{
					Name:   "Foo",
					Supers: []*types.Class{},
					Fields: map[string]types.Member{
						"x": {
							Access: types.AccessPublic,
							Type:   &tI64,
						},
					},
					Statics: map[string]types.Member{},
					Methods: map[string]types.Member{
						"foo": {
							Access: types.AccessPublic,
							Type: &types.Func{
								Args: []types.Type{&foo},
								Ret:  &tI64,
							},
						},
					},
				}
				return []types.Type{&foo}
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
				fooUnion := types.Union{Name: "Foo", Variants: fooSet}
				return []types.Type{
					&types.Union{
						Name:     "Foo",
						Variants: fooSet,
					},
					&types.Func{
						Args: []types.Type{&fooUnion},
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
								Supers: []*types.Class{},
								Fields: map[string]types.Member{
									"x": {
										Access: types.AccessPublic,
										Type:   &tI64,
									},
								},
								Statics: map[string]types.Member{},
								Methods: map[string]types.Member{},
							},
						},
					},
					Ret: &types.Generic{ID: 1},
				},
				&types.Func{
					Args: []types.Type{},
					Ret: &types.Record{
						Fields: map[string]types.Type{
							"x": &tI64,
						},
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
					Fields: map[string]types.Type{},
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
		//
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
						Args: []types.Type{grade, status},
						Ret: &types.Record{
							Fields: map[string]types.Type{
								"res":    &tI64,
								"grade":  grade,
								"status": status,
							},
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
			desc:  "extern declaration",
			input: "(extern def doThing (Int Str) [n])",
			typ: []types.Type{
				&types.Func{
					Args: []types.Type{
						&tI64,
					},
					Ret: &types.String{},
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
					Fields: map[string]types.Type{
						"y": &tI64,
					},
				}
				foo := types.Record{
					Fields: map[string]types.Type{
						"x": &bar,
					},
				}
				return []types.Type{
					&types.Func{
						Args: []types.Type{&foo},
						Ret:  &bar,
					},
					&foo,
					&bar,
				}
			}(),
		},
		// // currently broken, due to polarity.
		// // Fix would be to enforce method signatures and pre-type classes
		// // fully so that no (unbound) variables exist in method types
		// {
		// 	desc: "out of order class definition works",
		// 	input: `
		// 		(def testing (Foo Int) [foo] (foo#addOne))
		// 		(class Foo {
		// 			x I64,
		// 			pub (def addOne [self] (+ self.x 1)),
		// 		})
		// 	`,
		// 	typ: func() []types.Type {
		// 		var foo types.Class
		// 		foo = types.Class{
		// 			Name:   "Foo",
		// 			Supers: []*types.Class{},
		// 			Fields: map[string]types.Member{"x": {
		// 				Access: types.AccessPrivate, Type: &tI64}},
		// 			Statics: map[string]types.Member{},
		// 			Methods: map[string]types.Member{
		// 				"addOne": {
		// 					Access: types.AccessPublic,
		// 					Type: &types.Func{
		// 						Args: []types.Type{
		// 							&foo,
		// 						},
		// 						Ret: &tI64,
		// 					},
		// 				},
		// 			},
		// 		}
		// 		return []types.Type{
		// 			&types.Func{
		// 				Args: []types.Type{&foo},
		// 				Ret:  &tI64,
		// 			},
		// 			&foo,
		// 		}
		// 	}(),
		// },
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
				i32 := &types.Int{Signed: true, BitSize: 32}
				binding := &types.Class{}
				*binding = types.Class{
					Name:    "Binding",
					Supers:  []*types.Class{},
					Fields:  map[string]types.Member{},
					Statics: map[string]types.Member{},
					Methods: map[string]types.Member{
						"dupSource": {
							Access: 0,
							Type: &types.Func{
								Args: []types.Type{binding},
								Ret:  i32,
							},
						},
						"dupTarget": {
							Access: 0,
							Type: &types.Func{
								Args: []types.Type{binding},
								Ret:  i32,
							},
						},
					},
					Top: false,
				}
				return []types.Type{
					&types.Func{Args: []types.Type{binding}, Ret: i32},
					&types.Func{Args: []types.Type{binding}, Ret: i32},
					binding,
				}
			}(),
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			EnableTrace = true
			assert := assert.NewTestAsserts(t)
			checker := NewTyper("MainModule", true)

			program, err := parse.ParseString(tC.input)
			assert.Ok(err)

			ty, _, err := checker.TypeProgram(program)
			assert.Ok(err)

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
			for expect, got := range fun.ZipIter(slices.Values(tC.typ), slices.Values(typ)) {
				if !types.StructuralEq(expect, got) {
					t.Errorf("expected type\n  %v\ngot:\n  %v", types.DeepPrint(expect), types.DeepPrint(got))
				}
			}
		})
	}
}

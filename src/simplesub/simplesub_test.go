package simplesub

import (
	"errors"
	"slices"
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/fun"
	orderedset "github.com/horriblename/typee/src/internal/ordered_set"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/types"
)

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
			typ:   &types.Int{},
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
					"x": &types.Int{},
					"y": &types.Bool{},
				},
			},
		},
		{
			desc:  "if expr",
			input: "(if [true] 32 5)",
			typ:   &types.Int{},
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
				Ret:  &types.Int{},
			},
		},
		{
			desc:  "type annotated function",
			input: "(fn (Int Int) [x] 12)",
			typ: &types.Func{
				Args: []types.Type{&types.Int{}},
				Ret:  &types.Int{},
			},
		},
		{
			desc:  "application",
			input: "((fn [x] x) 34)",
			typ:   &types.Int{},
		},
		{
			desc:  "simple let expr",
			input: "(let [x 34 y 24] (if [true] x y))",
			typ:   &types.Int{},
		},
		// {
		// 	desc:  "local let expr does not generalize",
		// 	input: "(let [f (fn [x] x)] (let [y (f 3)] {f: f, y: y}))",
		// },
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			assert := assert.NewTestAsserts(t)
			checker := NewTyper(true)

			program, err := parse.ParseString(tC.input)
			assert.Ok(err)

			ty, err := checker.TypeTerm(&context{map[int]TypeScheme{}}, program[0])
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
				&types.Int{},
				&types.String{},
			},
		},
		{
			desc: "class definition",
			input: `
				(class Foo {x Int, pub (def id (Int Int) [x] x)})
				(def foo (Foo Foo) [f] f)
				(def main []
					(foo (Foo.new)))
			`,
			typ: []types.Type{
				&types.Class{
					Name:   "Foo",
					Supers: []*types.Class{},
					Fields: map[string]types.Member{
						"x": {
							Access: types.AccessPrivate,
							Type:   &types.Int{},
						},
					},
					Statics: map[string]types.Member{},
					Methods: map[string]types.Member{
						"id": {
							Access: types.AccessPublic,
							Type: &types.Func{
								Args: []types.Type{&types.Int{}},
								Ret:  &types.Int{},
							},
						},
					},
				},
				&types.Func{
					Args: []types.Type{&types.Class{Name: "Foo"}},
					Ret:  &types.Class{Name: "Foo"},
				},
				&types.Func{
					Args: []types.Type{},
					Ret:  &types.Class{Name: "Foo"},
				},
			},
		},
		{
			desc:  "self and Self alias",
			input: "(class Foo {pub x Int, pub (def foo  [self] self.x)})",
			typ: []types.Type{&types.Class{
				Name:   "Foo",
				Supers: []*types.Class{},
				Fields: map[string]types.Member{
					"x": {
						Access: types.AccessPrivate,
						Type:   &types.Int{},
					},
				},
				Statics: map[string]types.Member{},
				Methods: map[string]types.Member{
					"id": {
						Access: types.AccessPublic,
						Type: &types.Func{
							Args: []types.Type{
								&types.Class{Name: "Foo"},
								&types.Int{},
							},
							Ret: &types.Int{},
						},
					},
				},
			},
			},
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
						Ret:  &types.Int{},
					},
					&types.Func{
						Args: []types.Type{},
						Ret:  &types.Int{},
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
					Args: []types.Type{&types.Int{}},
					Ret:  &types.Int{},
				},
				&types.Func{
					Args: []types.Type{&types.Int{}},
					Ret:  &types.Int{},
				},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			EnableTrace = true
			assert := assert.NewTestAsserts(t)
			checker := NewTyper(true)

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

			t.Logf("coalesced type: %v\n", typ)
			assert.NEq(tC.typ, nil, "bad test case")
			if len(tC.typ) != len(typ) {
				t.Errorf("expected %d results, got %d", len(tC.typ), len(typ))
			}
			for expect, got := range fun.ZipIter(slices.Values(tC.typ), slices.Values(typ)) {
				if !types.StructuralEq(expect, got) {
					t.Errorf("expected type %v got: %v", expect, got)
				}
			}
		})
	}
}

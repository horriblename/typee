package simplesub

import (
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/parse"
)

func TestRecursiveGrouping(t *testing.T) {
	testCases := []struct {
		desc           string
		input          string
		groups         [][]string
		selfRecursives map[string]unit
	}{
		{
			desc:           "non-recursive",
			input:          "(def foo [x] (+ x 1))",
			groups:         [][]string{{"foo"}},
			selfRecursives: map[string]unit{},
		},
		{
			desc:           "self recursive",
			input:          "(def foo [x] (if [(= x 0)] 1 (foo (- x 1))))",
			groups:         [][]string{{"foo"}},
			selfRecursives: map[string]unit{"foo": {}},
		},
		{
			desc: "mutual recursive",
			input: `
				(def foo [x] (if [(= x 0)] 1 (bar (- x 1))))
				(def bar [x] (if [(= x 0)] 2 (foo x)))
			`,
			groups:         [][]string{{"foo", "bar"}},
			selfRecursives: map[string]unit{},
		},
		{
			desc: "is ordered by dependency",
			input: `
				(def id2 [x] (if [false] (id2 x) (c x)))
				(def foo [x] (if [(id2 (= x 0))] (id 1) (bar (- x 1))))
				(def bar [x] (if [(= x 0)] 2 (foo x)))
				;; these calls are named and ordered messily to ensure there is
				;; no correlation between name/definition location and scc groups
				;; order dependency chain is as follows:
				;;
				;; bar <-> foo -> id2(<->self) -> c -> d -> a -> b
				;;            \                                /
				;;             --> id <------------------------
				(def d [x] (a x))
				(def c [x] (d x))
				(def b [x] (id x))
				(def a [x] (b x))
				(def id [x] x)
			`,
			groups:         [][]string{{"id"}, {"b"}, {"a"}, {"d"}, {"c"}, {"id2"}, {"foo", "bar"}},
			selfRecursives: map[string]unit{"id2": {}},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			assert := assert.NewTestAsserts(t)

			ast, err := parse.ParseString(tC.input)
			assert.Ok(err, "parse error")

			groups, selfRecursives := groupRecursives(ast)

			// NOTE: scc group ordering is non-deterministic for functions that have no
			// dependency relation between each other, so avoid writing tests with such
			// functions e.g. (foo [x] (+ x 1)) and (bar [x] x) have no relation to each
			// other, there's no guarantee which will appear first.
			assert.DeepEq(
				fun.Map(tC.groups, sliceToSet),
				fun.Map(groups, sliceToSet),
				"different groups?",
			)
			assert.DeepEq(tC.selfRecursives, selfRecursives, "different self recursives")
		})
	}
}

func TestSortTypeDefs(t *testing.T) {
	testCases := []struct {
		desc           string
		input          string
		groups         [][]string
		selfRecursives map[string]unit
	}{
		{
			desc: "referring to other type def",
			input: `
				(type T1 Int)
				(type T2 (fn [T3] Int))
				(type T3 {x: T1})
				(class T4 {x T2})
				(class T6 (T5) {z Int})
				(class T5 {(def foo (T4) [] (T4.new))})
			`,
			groups:         [][]string{{"Int"}, {"T1"}, {"T3"}, {"T2"}, {"T4"}, {"T5"}, {"T6"}},
			selfRecursives: map[string]unit{},
		},
		{
			desc: "self recursives",
			input: `
				(class T1 {
					x Int,
					(def foo (Int T1) [x] (T1.new))
				})
			`,
			groups:         [][]string{{"Int"}, {"T1"}},
			selfRecursives: map[string]unit{"T1": unit{}},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			assert := assert.NewTestAsserts(t)

			ast, err := parse.ParseString(tC.input)
			assert.Ok(err, "parse error")

			groups, _, selfRecs := sortTypeDefs(ast)

			// NOTE: scc group ordering is non-deterministic for functions that have no
			// dependency relation between each other, so avoid writing tests with such
			// functions e.g. (foo [x] (+ x 1)) and (bar [x] x) have no relation to each
			// other, there's no guarantee which will appear first.
			assert.DeepEq(
				fun.Map(tC.groups, sliceToSet),
				fun.Map(groups, sliceToSet),
				"different groups?",
			)
			assert.DeepEq(tC.selfRecursives, selfRecs, "different self recursives")
		})
	}

}

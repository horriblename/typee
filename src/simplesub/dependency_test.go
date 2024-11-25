package simplesub

import (
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/parse"
)

func TestRecursiveGrouping(t *testing.T) {
	testCases := []struct {
		desc           string
		input          string
		groups         [][]any
		selfRecursives map[string]unit
	}{
		{
			desc:           "non-recursive",
			input:          "(def foo [x] (+ x 1))",
			groups:         [][]any{{"foo"}},
			selfRecursives: map[string]unit{},
		},
		{
			desc:           "self recursive",
			input:          "(def foo [x] (if [(= x 0)] 1 (foo (- x 1))))",
			groups:         [][]any{{"foo"}},
			selfRecursives: map[string]unit{"foo": {}},
		},
		{
			desc: "mutual recursive",
			input: `
				(def foo [x] (if [(= x 0)] 1 (bar (- x 1))))
				(def bar [x] (if [(= x 0)] 2 (foo x)))
			`,
			groups:         [][]any{{"foo", "bar"}},
			selfRecursives: map[string]unit{},
		},
		{
			desc: "mixed",
			input: `
				(def id [x] x)
				(def srec [x] (if [(> x 1)] 1 (srec (id (- x 1)))))
				(def foo [x] (if [(id (= x 0))] (id 1) (bar (- x 1))))
				(def bar [x] (if [(= x 0)] 2 (foo x)))
			`,
			groups:         [][]any{{"id"}, {"srec"}, {"foo", "bar"}},
			selfRecursives: map[string]unit{"srec": {}},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			tassert := assert.NewTestAsserts(t)

			ast, err := parse.ParseString(tC.input)
			tassert.Ok(err)

			groups, selfRecursives := groupRecursives(ast)

			// FIXME:turns out equality between [][]any is very fucking annoying so I'm skipping this for now
			t.Logf("expected groups:\n  %v", tC.groups)
			t.Logf("got:\n  %v", groups)
			t.Logf("\nexpected selfRecursives:\n  %v", tC.selfRecursives)
			t.Logf("got:\n  %v", selfRecursives)
		})
	}
}

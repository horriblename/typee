package types

import (
	"testing"

	"github.com/horriblename/typee/src/assert"
)

func TestStructuralEq(t *testing.T) {
	testCases := []struct {
		desc   string
		a      Type
		b      Type
		expect bool
	}{
		{
			desc:   "simple types",
			a:      &Int{},
			b:      &Int{},
			expect: true,
		},
		{
			desc:   "different simple types",
			a:      &String{},
			b:      &Bool{},
			expect: false,
		},
		{
			desc:   "same func",
			a:      &Func{Args: []Type{&Bool{}}, Ret: &Int{}},
			b:      &Func{Args: []Type{&Bool{}}, Ret: &Int{}},
			expect: true,
		},
		{
			desc:   "different func",
			a:      &Func{Args: []Type{&String{}}, Ret: &Int{}},
			b:      &Func{Args: []Type{&Bool{}}, Ret: &Int{}},
			expect: false,
		},
		{
			desc:   "same generic id",
			a:      &Generic{ID: 2},
			b:      &Generic{ID: 2},
			expect: true,
		},
		{
			desc:   "same structure generic",
			a:      &Generic{ID: 2},
			b:      &Generic{ID: 5},
			expect: true,
		},
		{
			desc:   "nested same structure",
			a:      &Func{Args: []Type{&Bool{}}, Ret: &Generic{ID: 3}},
			b:      &Func{Args: []Type{&Bool{}}, Ret: &Generic{ID: 9}},
			expect: true,
		},
		{
			desc:   "nested different structure",
			a:      &Func{Args: []Type{&Generic{ID: 10}}, Ret: &Generic{ID: 3}},
			b:      &Func{Args: []Type{&Bool{}}, Ret: &Generic{ID: 9}},
			expect: false,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			tassert := assert.NewTestAsserts(t)

			got := StructuralEq(tC.a, tC.b)
			tassert.Eq(got, tC.expect, "expected", tC.expect, "got", got)
		})
	}
}

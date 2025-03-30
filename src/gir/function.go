package gir

import (
	"slices"

	"github.com/linuxdeepin/go-gir/generator/gi"
)

type funcBuilder struct {
	fn        *gi.FunctionInfo
	orig_args []*gi.ArgInfo
	args      []funcBuilderArg
	rets      []funcBuilderArg
	skiplist  []int
}

type funcBuilderArg struct {
	// -1 means the "main" return value (including ctor return value)
	// -2 means error value
	// >= 0 means an output argument
	index int

	argInfo  *gi.ArgInfo
	typeInfo *gi.TypeInfo
}

func newFunctionBuilder(fi *gi.FunctionInfo) *funcBuilder {
	fb := new(funcBuilder)
	fb.fn = fi

	// prepare an array of ArgInfos
	for i, n := 0, fi.NumArg(); i < n; i++ {
		arg := fi.Arg(i)
		fb.orig_args = append(fb.orig_args, arg)
	}

	// build skip list
	for _, arg := range fb.orig_args {
		// TODO: why skip dynamic array?
		// ti := arg.Type()
		//
		// len := ti.ArrayLength()
		// if len != -1 {
		// 	skiplist = append(skiplist, len)
		// }

		clo := arg.Closure()
		if clo != -1 {
			fb.skiplist = append(fb.skiplist, clo)
		}

		des := arg.Destroy()
		if des != -1 {
			fb.skiplist = append(fb.skiplist, des)
		}
	}

	// then walk over arguments
	for i, ai := range fb.orig_args {
		if slices.Contains(fb.skiplist, i) {
			continue
		}

		ti := ai.Type()

		switch ai.Direction() {
		case gi.DIRECTION_IN:
			fb.args = append(fb.args, funcBuilderArg{i, ai, ti})
		case gi.DIRECTION_INOUT:
			fb.args = append(fb.args, funcBuilderArg{i, ai, ti})
			fb.rets = append(fb.rets, funcBuilderArg{i, ai, ti})
		case gi.DIRECTION_OUT:
			fb.rets = append(fb.rets, funcBuilderArg{i, ai, ti})
		}
	}

	// add return value if it exists to 'rets'
	if ret := fi.ReturnType(); ret != nil && ret.Tag() != gi.TYPE_TAG_VOID {
		fb.rets = append(fb.rets, funcBuilderArg{-1, nil, ret})
	}

	// add GError special argument (if any)
	if fi.Flags()&gi.FUNCTION_THROWS != 0 {
		// fb.rets = append(fb.rets, funcBuilderArg{-2, nil, nil})
	}

	return fb
}

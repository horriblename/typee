package gir

import "github.com/linuxdeepin/go-gir/generator/gi"

type funcBuilder struct {
	fn *gi.FunctionInfo
}

func newFuncBuilder(fn *gi.FunctionInfo) funcBuilder {
	return funcBuilder{fn}
}

package genqbe

import (
	"io"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/genqbe/qbeil"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/solve"
	"github.com/horriblename/typee/src/types"
)

type ctx struct {
	il        qbeil.Builder
	topLevels solve.SymbolTable
}

func Gen(w io.Writer, types solve.SymbolTable, ast []parse.Expr) {
	// top-levels
	ctx := ctx{qbeil.Builder{Writer: w}, types}
	for _, expr := range ast {
		gen(ctx, expr)
	}
}

func gen(ctx ctx, expr parse.Expr) {
	switch e := expr.(type) {
	case *parse.IntLiteral:
		panic("idk")
	case *parse.FuncDef:
		genFunc(ctx, e)
	}
}

func genFunc(ctx ctx, expr *parse.FuncDef) {
	funcTyp, ok := ctx.topLevels[expr.Name].(*types.Func)
	assert.True(ok, "generate function code: type of ", expr.Name, " is not function")
	assert.Eq(len(expr.Args), len(funcTyp.Args))

	args := make([]qbeil.TypedVar, 0, len(expr.Args))

	for i, arg := range funcTyp.Args {
		args = append(args, qbeil.NewTypedVar(
			toILType(arg),
			expr.Args[i],
		))
	}

	assert.Ok(ctx.il.Func(toILType(funcTyp.Ret), expr.Name, args))
	for _, stmt := range expr.Body {
		gen(ctx, stmt)
	}
	ctx.il.EndFunc()
}

func toILType(typ types.Type) qbeil.BaseType {
	switch typ.(type) {
	case *types.Int:
		return qbeil.Long
	case *types.Bool:
		return qbeil.Word
	}

	panic("unimpl: conversion to IL of type " + typ.String())
}

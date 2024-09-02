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

func gen(ctx ctx, expr parse.Expr) (val qbeil.Value) {
	switch e := expr.(type) {
	case *parse.IntLiteral:
		panic("idk")
	case *parse.FuncDef:
		return genFunc(ctx, e)
	}

	panic("unimpl gen " + expr.Pretty())
}

func genFunc(ctx ctx, expr *parse.FuncDef) (val qbeil.Value) {
	funcTyp, ok := ctx.topLevels[expr.Name].(*types.Func)
	assert.True(ok, "generate function code: type of ", expr.Name, " is not function")
	assert.Eq(len(expr.Args), len(funcTyp.Args))
	assert.GreaterThan(len(expr.Body), 0, "function", expr.Name, "has empty body")

	variable := qbeil.Var{Global: true, Name: expr.Name}
	args := make([]qbeil.TypedVar, 0, len(expr.Args))

	for i, arg := range funcTyp.Args {
		args = append(args, qbeil.NewTypedVar(
			toILType(arg),
			expr.Args[i],
		))
	}

	linkage := qbeil.Linkage{}
	if expr.Name == "main" {
		linkage.Type = qbeil.Export
	}

	var ret qbeil.Value
	retTyp := toILType(funcTyp.Ret)
	assert.Ok(ctx.il.Func(linkage, &retTyp, variable.IL(), args))
	for _, stmt := range expr.Body[:len(expr.Body)-1] {
		ret = gen(ctx, stmt)
	}

	ctx.il.Ret(ret)
	ctx.il.EndFunc()
	return variable
}

func toILType(typ types.Type) qbeil.Type {
	switch typ.(type) {
	case *types.Int:
		return qbeil.Long
	case *types.Bool:
		return qbeil.Word
	default:
		panic("unimpl: conversion to IL of type " + typ.String())
	}
}

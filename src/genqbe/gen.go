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
		gen(&ctx, expr)
	}
}

func gen(ctx *ctx, expr parse.Expr) (val qbeil.Value) {
	switch e := expr.(type) {
	case *parse.IntLiteral:
		return qbeil.IntLiteral{Value: e.Number}
	case *parse.FuncDef:
		return genFunc(ctx, e)
	case *parse.Form:
		return genCall(ctx, e)
	case *parse.Symbol:
		// FIXME: no globals
		return qbeil.Var{Global: false, Name: e.Name}
	}

	panic("unimpl gen " + expr.Pretty())
}

func genFunc(ctx *ctx, expr *parse.FuncDef) (val qbeil.Value) {
	funcTyp, ok := ctx.topLevels[expr.Name].(*types.Func)
	assert.True(ok, "generate function code: type of ", expr.Name, " is not function")
	assert.Eq(len(expr.Args), len(funcTyp.Args))
	assert.GreaterThan(len(expr.Body), 0, "function", expr.Name, "has empty body")

	thisFunc := qbeil.Var{Global: true, Name: expr.Name}
	args := make([]qbeil.TypedVar, 0, len(expr.Args))

	for i, arg := range funcTyp.Args {
		args = append(args, qbeil.NewTypedVar(
			toILType(arg),
			qbeil.Var{Global: false, Name: expr.Args[i]},
		))
	}

	linkage := qbeil.Linkage{}
	if expr.Name == "main" {
		linkage.Type = qbeil.Export
	}

	retTyp := toILType(funcTyp.Ret)

	assert.Ok(ctx.il.Func(linkage, &retTyp, thisFunc.IL(), args))

	for _, stmt := range expr.Body[:len(expr.Body)-1] {
		gen(ctx, stmt)
	}

	ret := gen(ctx, expr.Body[len(expr.Body)-1])

	ctx.il.Ret(ret)
	ctx.il.EndFunc()
	return thisFunc
}

func genCall(ctx *ctx, expr *parse.Form) qbeil.Value {
	assert.GreaterThan(len(expr.Children), 0, "empty form")

	switch callee := expr.Children[0].(type) {
	case *parse.Symbol:
		switch callee.Name {
		case "+":
			assert.Eq(len(expr.Children), 3, "wrong function arg count")

			left := gen(ctx, expr.Children[1])
			right := gen(ctx, expr.Children[2])
			target := ctx.il.TempVar()
			ctx.il.Arithmetic(target.IL(), qbeil.Long, "add", left, right)

			return target
		default:
			panic("unimpl: genCall for callee named " + callee.Name)
		}
	default:
		panic("unimpl: genCall for callee of the form " + expr.Pretty())
	}
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

package genqbe

import (
	_ "embed"
	"fmt"
	"io"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/genqbe/qbeil"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/solve"
	"github.com/horriblename/typee/src/types"
)

//go:embed builtins.qbe
var builtinsQbe string

type ctx struct {
	il        qbeil.Builder
	topLevels solve.SymbolTable
	statics   map[qbeil.Var]string
	userTypes map[string]qbeil.StructType
}

func Gen(w io.Writer, types solve.SymbolTable, ast []parse.Expr) {
	fmt.Fprint(w, builtinsQbe)

	// top-levels
	ctx := ctx{qbeil.Builder{Writer: w}, types, map[qbeil.Var]string{},
		map[string]qbeil.StructType{}}

	ctx.userTypes["Str"] = qbeil.StructType{
		Name: "Str",
		Fields: []qbeil.Type{
			qbeil.Long, // pointer to string
			qbeil.Word, // size
		},
	}

	for _, expr := range ast {
		gen(&ctx, expr)
	}

	ctx.finish()
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
		// FIXME: bad global lookup
		if _, ok := ctx.topLevels[e.Name]; ok {
			return qbeil.Var{Global: true, Name: e.Name}
		}
		return qbeil.Var{Global: false, Name: e.Name}
	case *parse.StrLiteral:
		dataGlobal := ctx.il.TempVar(true)
		ctx.statics[dataGlobal] = fmt.Sprintf(`{b "%s"}`, e.Content)

		// the Str struct on stack
		strPtr := ctx.il.TempVar(false)
		// TODO: construct Str obj on stack Str{data: &dataGlobal, len: len(dataGlobal)}

		ctx.il.Arithmetic(strPtr.IL(), qbeil.Long, "alloc4", qbeil.IntLiteral{Value: 16 + 8})
		ctx.il.Command("storel", dataGlobal, strPtr)

		lenPtr := ctx.il.TempVar(false)
		// 64-bit system
		ctx.il.Arithmetic(lenPtr.IL(), qbeil.Long, "add", strPtr, qbeil.IntLiteral{Value: 8})
		ctx.il.Command("storel", qbeil.IntLiteral{Value: int64(len(e.Content))}, lenPtr)

		return strPtr
	case *parse.LetExpr:
		return genLet(ctx, e)
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
			ctx.toILType(arg),
			qbeil.Var{Global: false, Name: expr.Args[i]},
		))
	}

	linkage := qbeil.Linkage{}
	retTyp := ctx.toILType(funcTyp.Ret)
	if expr.Name == "main" {
		linkage.Type = qbeil.Export
		retTyp = qbeil.Word
	}

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
			target := ctx.il.TempVar(false)
			ctx.il.Arithmetic(target.IL(), qbeil.Long, "add", left, right)

			return target
		case "print":
			assert.Eq(len(expr.Children), 2, `wrong arg count for "print"`)
			arg := gen(ctx, expr.Children[1])
			target := ctx.il.TempVar(false)
			ctx.il.Call(&target, qbeil.Word,
				qbeil.Var{Global: true, Name: "print"},
				[]qbeil.TypedValue{{
					Type:  ctx.toILType(&types.String{}),
					Value: arg,
				}})

			return target
		default:
			// TODO: local functions
			fn, found := ctx.topLevels[callee.Name]
			assert.True(found, "function does not exist:", callee.Name)

			funcSig, ok := fn.(*types.Func)
			assert.True(ok, "tried to call non-function top-level:", callee.Name)
			assert.Eq(len(expr.Children), len(funcSig.Args)+1, callee.Name, ": function argument count does not match signature")
			target := ctx.il.TempVar(false)

			args := fun.ZipMap(expr.Children[1:], funcSig.Args, func(arg parse.Expr, typ types.Type) qbeil.TypedValue {
				return qbeil.TypedValue{Type: ctx.toILType(typ), Value: gen(ctx, arg)}
			})
			funcVar := qbeil.Var{Global: true, Name: callee.Name}

			ctx.il.Call(&target, ctx.toILType(funcSig.Ret), funcVar, args)

			return target
		}
	default:
		panic("unimpl: genCall for callee of the form " + expr.Pretty())
	}
}

func genLet(ctx *ctx, expr *parse.LetExpr) qbeil.Value {
	panic("unimpl: gen let")
}

func (ctx *ctx) finish() {
	for _, typ := range ctx.userTypes {
		_, err := ctx.il.Writer.Write([]byte(typ.Define()))
		assert.Ok(err)

		_, err = ctx.il.Writer.Write([]byte{'\n'})
		assert.Ok(err)
	}

	for name, data := range ctx.statics {
		_, err := ctx.il.Writer.Write([]byte("data "))
		assert.Ok(err)

		_, err = ctx.il.Writer.Write([]byte(name.IL()))
		assert.Ok(err)

		_, err = ctx.il.Writer.Write([]byte(" = "))
		assert.Ok(err)

		_, err = ctx.il.Writer.Write([]byte(data))
		assert.Ok(err)

		_, err = ctx.il.Writer.Write([]byte{'\n'})
		assert.Ok(err)
	}
}

func (ctx *ctx) toILType(typ types.Type) qbeil.Type {
	switch typ.(type) {
	case *types.Int:
		return qbeil.Long
	case *types.Bool:
		return qbeil.Word
	case *types.String:
		return ctx.userTypes["Str"]
	default:
		panic("unimpl: conversion to IL of type " + typ.String())
	}
}

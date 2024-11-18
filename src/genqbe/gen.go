package genqbe

import (
	_ "embed"
	"errors"
	"fmt"
	"io"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/genqbe/qbeil"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/simplesub"
	"github.com/horriblename/typee/src/types"
)

//go:embed builtins.qbe
var builtinsQbe string

// compiler bugs
var ErrCannotCompilePolymorphicType = errors.New("tried to compile a polymorphic type")

type ctx struct {
	ptrType      qbeil.BaseType
	intType      qbeil.BaseType
	defaultAlign int

	il         qbeil.Builder
	types      map[int]simplesub.TypeScheme
	simplified map[int]types.Type
	statics    map[qbeil.Var]string
	globals    map[string]int
	userTypes  map[string]qbeil.StructType
}

func Gen(w io.Writer, typs map[int]simplesub.TypeScheme, ast []parse.Expr) {
	ctx := ctx{
		qbeil.Long, // TODO: infer ptr & int size + manual options
		qbeil.Long,
		64,
		qbeil.Builder{OutFile: w},
		typs,
		map[int]types.Type{},
		map[qbeil.Var]string{},
		globals(ast),
		map[string]qbeil.StructType{},
	}

	ctx.userTypes["Str"] = qbeil.StructType{
		Align:  0,
		Name:   "Str",
		Fields: []qbeil.Type{ctx.ptrType, ctx.ptrType},
	}

	// struct GObject {
	//  GTypeInstance g_type_instance
	//  guint ref_count; /* atomic */
	//  GData* qdata
	// }
	//
	// struct GTypeInstance {
	//  GTypeClass* g_class;
	// }
	ctx.userTypes["GObject"] = qbeil.StructType{
		Name: "GObject",
		Fields: []qbeil.Type{
			ctx.ptrType,
			ctx.intType,
			ctx.ptrType,
		},
	}

	for _, expr := range ast {
		genTopLevel(&ctx, expr)
	}

	ctx.finish()
}

func genTopLevel(ctx *ctx, expr parse.Expr) {
	switch e := expr.(type) {
	case *parse.ClassDef:
		ctx.userTypes[e.Name] = qbeil.StructType{
			Align:  0,
			Name:   e.Name,
			Fields: []qbeil.Type{},
		}
	case *parse.Set:
		gen(ctx, expr)
	case *parse.FuncDef:
		gen(ctx, expr)
	default:
		panic(fmt.Sprintf("unexpected parse.Expr: %#v", e))
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
		if _, ok := ctx.globals[e.Name]; ok {
			return qbeil.Var{Global: true, Name: e.Name}
		}
		return qbeil.Var{Global: false, Name: e.Name}
	case *parse.StrLiteral:
		dataGlobal := ctx.il.TempVar(true)
		ctx.statics[dataGlobal] = fmt.Sprintf(`{b "%s"}`, e.Content)

		// the Str struct on stack
		strPtr := ctx.il.TempVar(false)

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
	funcTyp, ok := ctx.simplify(expr.ID()).(*types.Func)
	assert.True(ok, "generate function code: type of ", expr.Name, " is not function")
	assert.Eq(len(expr.Args), len(funcTyp.Args))
	assert.GreaterThan(len(expr.Body), 0, "function", expr.Name, "has empty body")

	thisFunc := qbeil.Var{Global: true, Name: expr.Name}
	argTyps := make([]qbeil.TypedVar, 0, len(expr.Args))

	for i, argTyp := range funcTyp.Args {
		argTyps = append(argTyps, qbeil.NewTypedVar(
			ctx.toILType(argTyp),
			qbeil.Var{Global: false, Name: expr.Args[i]},
		))
	}

	linkage := qbeil.Linkage{}
	retTyp := ctx.toILType(funcTyp.Ret)
	if expr.Name == "main" {
		linkage.Type = qbeil.Export
		retTyp = qbeil.Word
	}

	assert.Ok(ctx.il.Func(linkage, &retTyp, thisFunc.IL(), argTyps))

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
	case *parse.New:
		classIL := ctx.userTypes[callee.Class]
		bits, _ := ctx.sizeOf(classIL)

		val := ctx.il.TempVar(false)
		ctx.il.Call(
			&val,
			ctx.ptrType,
			qbeil.Var{Global: true, Name: "malloc"},
			[]qbeil.TypedValue{{
				Type:  ctx.intType,
				Value: qbeil.IntLiteral{Value: int64(bits)}},
			},
		)

		return val

	case *parse.MethodAccess:
		panic("TODO: gen method call")

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
			fn := ctx.simplify(callee.ID())

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
		_, err := ctx.il.OutFile.Write([]byte(typ.Define()))
		assert.Ok(err)

		_, err = ctx.il.OutFile.Write([]byte{'\n'})
		assert.Ok(err)
	}

	for name, data := range ctx.statics {
		_, err := ctx.il.OutFile.Write([]byte("data "))
		assert.Ok(err)

		_, err = ctx.il.OutFile.Write([]byte(name.IL()))
		assert.Ok(err)

		_, err = ctx.il.OutFile.Write([]byte(" = "))
		assert.Ok(err)

		_, err = ctx.il.OutFile.Write([]byte(data))
		assert.Ok(err)

		_, err = ctx.il.OutFile.Write([]byte{'\n'})
		assert.Ok(err)
	}

	fmt.Fprint(ctx.il.OutFile, builtinsQbe)
	io.Copy(ctx.il.OutFile, &ctx.il.Writer)
}

func (ctx *ctx) toILType(typ types.Type) qbeil.Type {
	switch t := typ.(type) {
	case *types.Int:
		return ctx.intType
	case *types.Bool:
		return ctx.intType
	case *types.String:
		return ctx.userTypes["Str"]
	case *types.Class:
		if t.Name == "" {
			panic("unnamed classes should be illegal at codegen")
		}

		if ut, ok := ctx.userTypes[t.Name]; ok {
			return ut
		}

		if len(t.Supers) == 0 {
			fields := []qbeil.Type{
				ctx.userTypes["GObject"], // parent
				ctx.ptrType,              // private pointer
			}
			for _, field := range t.Fields {
				if field.Access == types.AccessPublic || field.Access == types.AccessProtected {
					fields = append(fields, ctx.fieldToILType(field.Type))
				}
			}
			ctx.userTypes[t.Name] = qbeil.StructType{
				Align:  0,
				Name:   t.Name,
				Fields: fields,
			}
		}
		panic("unimpl: super types")

	default:
		panic("unimpl: conversion to IL of type " + typ.String())
	}
}

func (ctx *ctx) fieldToILType(field types.Type) qbeil.Type {
	switch t := field.(type) {
	case *types.Class:
		return ctx.ptrType
	case *types.Bool, *types.Int, *types.String:
		return ctx.toILType(field)
	default:
		panic(fmt.Sprintf("unexpected types.Type: %#v", t))
	}
}

func (ctx *ctx) simplify(exprID int) types.Type {
	// this cache probably isn't that helpful? but caching by simplesub.Type is
	// probably too costly to hash
	if t, ok := ctx.simplified[exprID]; ok {
		return t
	}

	st, ok := ctx.types[exprID].(simplesub.SimpleType)
	if !ok {
		panic("compiler bug: " + ErrCannotCompilePolymorphicType.Error())
	}
	t1 := simplesub.SimplifyType(st)
	t2 := simplesub.CoalesceType(t1)
	ctx.simplified[exprID] = t2
	return t2
}

func (self *ctx) sizeOf(t qbeil.Type) (bits int, align int) {
	switch t := t.(type) {
	case qbeil.BaseType:
		switch t {
		case qbeil.Word:
			return 32, self.defaultAlign
		case qbeil.Long:
			return 64, self.defaultAlign
		case qbeil.Single:
			return 32, self.defaultAlign
		case qbeil.Double:
			return 64, self.defaultAlign
		default:
			panic(fmt.Sprintf("unexpected qbeil.BaseType: %#v", t))
		}
	case qbeil.StructType:
		bits := 0
		align := 0
		for _, field := range t.Fields {
			// TODO: actually handle align
			fs, fa := self.sizeOf(field)
			align = max(align, fa)
			bits += fs
		}
		return bits, align
	default:
		panic(fmt.Sprintf("unexpected qbeil.Type: %#v", t))
	}
}

func globals(program []parse.Expr) map[string]int {
	// TODO: probably don't need this
	vars := map[string]int{}
	for _, e := range program {
		switch et := e.(type) {
		case *parse.Set:
			vars[et.Name] = et.ID()
		case *parse.FuncDef:
			vars[et.Name] = et.ID()
		case *parse.ClassDef:

		default:
			panic(fmt.Sprintf("illegal global expression: %v", e.Pretty()))
		}
	}

	return vars
}

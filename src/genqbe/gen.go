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
		genClassDef(ctx, e)

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
		return genFunc(ctx, "", e)
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

// class should be empty string for non-methods
func genFunc(ctx *ctx, class string, expr *parse.FuncDef) (val qbeil.Value) {
	friendlyName := fmt.Sprintf("%s.%s", class, expr.Name)
	funcTyp, ok := ctx.simplify(expr.ID()).(*types.Func)
	assert.True(ok, "generate function code: type of ", friendlyName, " is not function")
	assert.Eq(len(expr.Args), len(funcTyp.Args))
	assert.GreaterThan(len(expr.Body), 0, "function", friendlyName, "has empty body")

	mangled := mangleName(mangleOpts{class: class, name: expr.Name})

	thisFunc := qbeil.Var{Global: true, Name: mangled}
	argTyps := make([]qbeil.TypedVar, 0, len(expr.Args))

	for i, argTyp := range funcTyp.Args {
		argTyps = append(argTyps, qbeil.NewTypedVar(
			ctx.toILType(argTyp),
			qbeil.Var{Global: false, Name: expr.Args[i]},
		))
	}

	linkage := qbeil.Linkage{}
	retTyp := ctx.toILType(funcTyp.Ret)
	if expr.Name == "main" && class == "" {
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
		// FIXME: callee.Class is not the class name, but the object name
		return genCallWithFuncName(ctx, callee.Class, callee.Method, expr)

	case *parse.Symbol:
		return genCallWithFuncName(ctx, "", callee.Name, expr)

	default:
		panic("unimpl: genCall for callee of the form " + expr.Pretty())
	}
}

func genCallWithFuncName(ctx *ctx, class string, fnName string, expr *parse.Form) qbeil.Value {
	mangled := mangleName(mangleOpts{class: class, name: fnName})
	fnFriendlyName := fmt.Sprintf("%s.%s", class, fnName)
	callee := expr.Children[0]

	switch mangled {
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
		assert.True(ok, "tried to call non-function top-level:", fnFriendlyName)
		assert.Eq(len(expr.Children), len(funcSig.Args)+1, fnFriendlyName, ": function argument count does not match signature")
		target := ctx.il.TempVar(false)

		args := fun.ZipMap(expr.Children[1:], funcSig.Args, func(arg parse.Expr, typ types.Type) qbeil.TypedValue {
			return qbeil.TypedValue{Type: ctx.toILType(typ), Value: gen(ctx, arg)}
		})
		funcVar := qbeil.Var{Global: true, Name: mangled}

		ctx.il.Call(&target, ctx.toILType(funcSig.Ret), funcVar, args)

		return target
	}
}

func genLet(ctx *ctx, expr *parse.LetExpr) qbeil.Value {
	panic("unimpl: gen let")
}

func genClassDef(ctx *ctx, e *parse.ClassDef) {

	// TODO: support generics
	ct := ctx.simplify(e.ID())
	classTy, ok := ct.(*types.Class)
	if !ok {
		panic(fmt.Sprintf("compiler bug: class definition yields non-class type %#v", ct))
	}

	ctx.userTypes[e.Name] = ctx.classDefIL(classTy, e)
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
	switch typ.(type) {
	case *types.Int:
		return ctx.intType
	case *types.Bool:
		return ctx.intType
	case *types.String:
		return ctx.userTypes["Str"]
	case *types.Class:
		return ctx.ptrType

	default:
		panic("unimpl: conversion to IL of type " + typ.String())
	}
}

// like [ctx.toILType] but converts class type to a pointer instead of its full [qbeil.StructType]
func (ctx *ctx) classDefIL(t *types.Class, e *parse.ClassDef) qbeil.StructType {
	if t.Name == "" {
		// FIXME: generic support
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
				fields = append(fields, ctx.toILType(field.Type))
			}
		}

		// generate methods
		for _, field := range e.Fields {
			method, ok := field.(parse.ClassMethod)
			if !ok {
				continue
			}

			genFunc(ctx, t.Name, method.Func)
		}

		return qbeil.StructType{
			Align:  0,
			Name:   t.Name,
			Fields: fields,
		}

	}
	panic("unimpl: super types")
}

func (ctx *ctx) simplify(exprID int) types.Type {
	// this cache probably isn't that helpful? but caching by simplesub.Type is
	// probably too costly to hash
	if t, ok := ctx.simplified[exprID]; ok {
		return t
	}

	st, ok := ctx.types[exprID].(simplesub.SimpleType)
	if !ok {
		pt, ok := ctx.types[exprID].(simplesub.PolymorphicType)
		// HACK: temp workaround for class and method types
		if !ok {
			panic(fmt.Sprintf("unreachable or nil TypeScheme: %v", ctx.types[exprID]))
		}
		st = pt.Body
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

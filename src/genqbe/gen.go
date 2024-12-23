package genqbe

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"strconv"

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

	typeDecl   bytes.Buffer
	il         qbeil.Builder
	types      map[int]simplesub.TypeScheme
	simplified map[int]types.Type
	statics    map[qbeil.Var]string
	globals    map[string]int
	userTypes  map[string]qbeil.AggregateType

	idGenerator          int64
	generatedTranslation map[types.Type]qbeil.AggregateType
}

func Gen(w io.Writer, typs map[int]simplesub.TypeScheme, ast []parse.Expr) {
	ctx := ctx{
		qbeil.Long, // TODO: infer ptr & int size + manual options
		qbeil.Long,
		64,
		bytes.Buffer{},
		qbeil.Builder{OutFile: w},
		typs,
		map[int]types.Type{},
		map[qbeil.Var]string{},
		globals(ast),
		map[string]qbeil.AggregateType{},
		0,
		map[types.Type]qbeil.AggregateType{},
	}

	ctx.declareType("Str", qbeil.StructType{
		Align:   0,
		Name:    "Str",
		Layouts: map[string]qbeil.FieldLayout{},
		Fields: []qbeil.RepeatType{
			qbeil.SingleType(ctx.ptrType),
			qbeil.SingleType(ctx.ptrType),
		},
	})

	// struct GObject {
	//  GTypeInstance g_type_instance
	//  guint ref_count; /* atomic */
	//  GData* qdata
	// }
	//
	// struct GTypeInstance {
	//  GTypeClass* g_class;
	// }
	ctx.declareType("GObject", qbeil.StructType{
		Name:    "GObject",
		Layouts: map[string]qbeil.FieldLayout{},
		Fields: []qbeil.RepeatType{
			qbeil.SingleType(ctx.ptrType),
			qbeil.SingleType(ctx.intType),
			qbeil.SingleType(ctx.ptrType),
		},
	})

	for _, expr := range ast {
		genTopLevel(&ctx, expr)
	}

	ctx.finish()
}

func genTopLevel(ctx *ctx, expr parse.Expr) {
	switch e := expr.(type) {
	case *parse.Import:
	case *parse.ObjectTypeDef:
		genClassDef(ctx, e)

	case *parse.UnionDef:
		genUnionDef(ctx, e)

	case *parse.Set:
		genGlobalVar(ctx, e)
	case *parse.FuncDef:
		gen(ctx, expr)
	case *parse.EnumDef, *parse.TypeAlias:

	default:
		panic(fmt.Sprintf("unexpected parse.Expr: %#v", e))
	}
}

func gen(ctx *ctx, expr parse.Expr) qbeil.Value {
	switch e := expr.(type) {
	case *parse.IntLiteral:
		return qbeil.IntLiteral{Value: e.Number}
	case *parse.FuncDef:
		return genFunc(ctx, "", e)
	case *parse.Form:
		return genCall(ctx, e)
	case *parse.EnumAccess:
		t := ctx.simplify(e.ID())
		enum := assert.Cast[*types.Enum](t, "codegen: enum access has non-enum left-hand side")

		val, ok := enum.Values[e.Key]
		assert.True(ok, "codegen: enum ", e.Enum, "has no variant", e.Key)

		return qbeil.IntLiteral{
			Value: val,
		}
	case *parse.Symbol:
		if _, ok := ctx.globals[e.Name]; ok {
			return qbeil.Var{Global: true, Name: e.Name}
		}
		return qbeil.Var{Global: false, Name: e.Name}

	case *parse.ExternCall:
		target := ctx.il.TempVar(false)

		argTypes := fun.Map(e.Args, func(arg parse.Expr) types.Type {
			return ctx.simplify(arg.ID())
		})
		args := fun.ZipMap(e.Args, argTypes, func(arg parse.Expr, typ types.Type) qbeil.TypedValue {
			return qbeil.TypedValue{Type: ctx.toILType(typ), Value: gen(ctx, arg)}
		})
		funcVar := qbeil.Var{Global: true, Name: e.Symbol.Name}

		retTy := ctx.simplify(e.ID())

		ctx.il.Call(&target, ctx.toILType(retTy), funcVar, args)

		return target

	case *parse.RecordAccess:
		ty := ctx.simplify(e.Record.ID())
		class := ty.(*types.Class)
		assert.Neq(class.Name, "", "unnamed class not yet supported")

		ct := ctx.userTypes[class.Name]
		classTy, ok := ct.(qbeil.StructType)
		assert.True(ok, "codegen: record access on non-struct type (type checker bug?)")

		fieldLayout, ok := classTy.Layouts[e.Field]
		if !ok {
			panic(fmt.Sprintf("typer bug: tried to use a non-existent class field %s.%s", class.Name, e.Field))
		}

		// TODO: once we support structs, we can't just pass this around (can we?)
		bt, ok := fieldLayout.Type.(qbeil.BaseType)
		if !ok {
			panic("unreachable: embedded struct types are currently invalid in classes")
		}

		// TODO: 32-bit system
		addr := ctx.il.TempVar(false)
		ctx.il.Arithmetic(addr.IL(), ctx.ptrType, "add",
			gen(ctx, e.Record),
			qbeil.IntLiteral{Value: int64(fieldLayout.Offset)},
		)

		val := ctx.il.TempVar(false)
		switch bt {
		case qbeil.Double:
			ctx.il.Arithmetic(val.IL(), bt, "loadd", addr)
		case qbeil.Long:
			ctx.il.Arithmetic(val.IL(), bt, "loadl", addr)
		case qbeil.Single:
			ctx.il.Arithmetic(val.IL(), bt, "loads", addr)
		case qbeil.Word:
			ctx.il.Arithmetic(val.IL(), bt, "loadw", addr)
		default:
			panic(fmt.Sprintf("unexpected qbeil.BaseType: %#v", bt))
		}

		return val

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

func genGlobalVar(ctx *ctx, expr *parse.Set) (val qbeil.Value) {
	switch val := expr.Value.(type) {
	case *parse.IntLiteral:
		return ctx.il.Data(
			qbeil.DataDef{
				Linkage: qbeil.Linkage{},
				VarName: expr.Name,
				Align:   0,
			},
			ctx.intType,
			qbeil.IntLiteral{
				Value: val.Number,
			})

	case *parse.StrLiteral:
		return ctx.il.StrData(qbeil.DataDef{
			Linkage: qbeil.Linkage{},
			VarName: expr.Name,
			Align:   0,
		}, val.Content)

	case *parse.FloatLiteral:
		return ctx.il.Data(
			qbeil.DataDef{
				Linkage: qbeil.Linkage{},
				VarName: expr.Name,
				Align:   0,
			},
			qbeil.Double,
			qbeil.FloatLiteral{
				Value: val.Number,
			})

	case *parse.BoolLiteral:
		var v int64 = 0
		if val.Value {
			v = 1
		}
		return ctx.il.Data(
			qbeil.DataDef{
				Linkage: qbeil.Linkage{},
				VarName: expr.Name,
				Align:   0,
			},
			ctx.toILType(ctx.simplify(val.ID())),
			qbeil.IntLiteral{
				Value: v,
			})

	default:
		panic(fmt.Sprintf("global variable of expression %s not supported", expr))
	}
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
				Value: qbeil.IntLiteral{Value: int64(bits / 8)}},
			},
		)

		return val

	case *parse.MethodAccess:
		ty := ctx.simplify(callee.Var.ID())
		class := ty.(*types.Class).Name
		assert.Neq(class, "", "unnamed class not yet supported")
		return genCallWithFuncName(ctx, "", class, callee.Method, expr)

	case *parse.RecordAccess:
		// TODO: currently only module access supported
		var modulePath string
		switch lhs := callee.Record.(type) {
		case *parse.Symbol:
			modulePath = lhs.Name
		case *parse.RecordAccess:
			modulePath = flattenRecordAccessPath(lhs)
		default:
			panic(fmt.Sprintf("unexpected lhs of record accessor %T in: %s", callee.Record, callee))
		}

		return genCallWithFuncName(ctx, modulePath, "", callee.Field, expr)

	case *parse.Symbol:
		return genCallWithFuncName(ctx, "", "", callee.Name, expr)

	default:
		panic("unimpl: genCall for callee of the form " + expr.Pretty())
	}
}

func genCallWithFuncName(ctx *ctx, module string, class string, fnName string, expr *parse.Form) qbeil.Value {
	mangled := mangleName(mangleOpts{module: module, class: class, name: fnName})
	fnFriendlyName := fnName
	if class != "" {
		fnFriendlyName = fmt.Sprintf("%s.%s", class, fnName)
	}
	callee := expr.Children[0]

	switch mangled {
	case "+":
		assert.Eq(len(expr.Children), 3, "wrong function arg count")

		left := gen(ctx, expr.Children[1])
		right := gen(ctx, expr.Children[2])
		target := ctx.il.TempVar(false)
		ctx.il.Arithmetic(target.IL(), qbeil.Long, "add", left, right)

		return target
	case "exit":
		assert.Eq(len(expr.Children), 2, "wrong function arg count")

		exitCode := gen(ctx, expr.Children[1])
		ctx.il.Call(nil, qbeil.Long, qbeil.Var{Global: true, Name: "exit"}, []qbeil.TypedValue{
			{Type: ctx.intType, Value: exitCode},
		})
		return qbeil.IntLiteral{Value: 0} // TODO: there's probably a better way
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
		assert.True(ok, "tried to call non-function top-level:", fnFriendlyName, "of type", fmt.Sprintf("%#v", fn))
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

func genClassDef(ctx *ctx, e *parse.ObjectTypeDef) {

	// TODO: support generics
	ct := ctx.simplify(e.ID())
	classTy, ok := ct.(*types.Class)
	if !ok {
		panic(fmt.Sprintf("compiler bug: class definition yields non-class type %#v", ct))
	}

	ctx.declareType(e.Name, ctx.classDefIL(classTy, e))
}

func genUnionDef(ctx *ctx, e *parse.UnionDef) {
	st := ctx.simplify(e.ID())
	unionTy, ok := st.(*types.Union)
	if !ok {
		panic(fmt.Sprintf("compiler bug: union definition yields non-class type %#v", st))
	}

	ctx.declareType(e.Name, ctx.unionDefIL(unionTy, e))
}

func (ctx *ctx) finish() {
	ctx.il.OutFile.Write(ctx.typeDecl.Bytes())

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
	io.Copy(ctx.il.OutFile, &ctx.il.Buf)
}

func (ctx *ctx) toILType(typ types.Type) qbeil.Type {
	switch t := typ.(type) {
	case *types.Int:
		return ctx.intType
	case *types.Float:
		return qbeil.Double
	case *types.Bool:
		return ctx.intType
	case *types.String:
		return ctx.userTypes["Str"]
	case *types.Class:
		return ctx.ptrType
	case *types.Record:
		return ctx.ptrType
	case *types.Ptr:
		return ctx.ptrType
	case *types.Union:
		if t.Name == "" {
			panic("unnamed union unsupported at code gen")
		}

		ut, ok := ctx.userTypes[t.Name]
		assert.True(ok, "during codegen: undefined union", t.Name)
		return ut

	case *types.Enum:
		return ctx.intType

	case *types.Array:
		if il, ok := ctx.generatedTranslation[t]; ok {
			return il
		}

		elTy := ctx.toILType(t.Type)
		name := ctx.newTempName(fmt.Sprintf("_array"))
		ilTyp := qbeil.StructType{
			Align:   0,
			Name:    name,
			Layouts: map[string]qbeil.FieldLayout{},
			Fields:  []qbeil.RepeatType{{Type: elTy, Count: int(t.Size)}},
		}
		ctx.declareType(name, ilTyp)
		ctx.generatedTranslation[t] = ilTyp
		return ilTyp

	case *types.Slice:
		// FIXME: should be a struct like this: {ptr: ptrType, size: int}
		return ctx.ptrType
	default:
		panic("unimpl: conversion to QBE IL from type " + typ.String())
	}
}

// like [ctx.toILType] but converts class type to a pointer instead of its full [qbeil.StructType]
func (ctx *ctx) classDefIL(t *types.Class, e *parse.ObjectTypeDef) qbeil.AggregateType {
	if t.Name == "" {
		// FIXME: generic support
		panic("unnamed classes should be illegal at codegen")
	}

	// FIXME: why did I put this here?? there's no way a userType already exists during class definition right?
	if ut, ok := ctx.userTypes[t.Name]; ok {
		return ut
	}

	if len(t.Supers) == 0 {
		fields := []qbeil.RepeatType{
			qbeil.SingleType(ctx.userTypes["GObject"]), // parent
			qbeil.SingleType(ctx.ptrType),              // private pointer
		}
		bits, _ := ctx.sizeOf(ctx.userTypes["GObject"]) // TODO: align
		pubOffset := bits / 8
		layouts := map[string]qbeil.FieldLayout{}

		for name, field := range t.Fields {
			if field.Access == types.AccessPublic || field.Access == types.AccessProtected {
				ilTy := ctx.toILType(field.Type)
				bits, _ := ctx.sizeOf(ilTy) // TODO: align
				fields = append(fields, qbeil.SingleType(ilTy))
				layouts[name] = qbeil.FieldLayout{
					Offset: pubOffset,
					Type:   ilTy,
				}
				pubOffset += bits / 8
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
			Align:   0,
			Layouts: layouts,
			Name:    t.Name,
			Fields:  fields,
		}

	}
	panic("unimpl: super types")
}

func (ctx *ctx) unionDefIL(t *types.Union, e *parse.UnionDef) qbeil.AggregateType {
	if t.Name == "" {
		panic("unnamed union currently unsupported at codegen")
	}

	variants := fun.Map(t.Variants.Slice(), func(t types.Type) qbeil.Type {
		return ctx.toILType(t)
	})

	ut := qbeil.NewUnionType(e.Name, 0, ctx.defaultAlign, variants)
	ctx.userTypes[e.Name] = ut
	return ut
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
			panic(fmt.Sprintf("unreachable or nil TypeScheme at expr ID %d: %v", exprID, ctx.types[exprID]))
		}
		st = pt.Body
	}
	t1 := simplesub.SimplifyType(st)
	t2 := simplesub.CoalesceType(t1)
	ctx.simplified[exprID] = t2
	return t2
}

func (ctx *ctx) declareType(name string, t qbeil.AggregateType) {
	ctx.userTypes[name] = t
	_, err := ctx.typeDecl.Write([]byte(t.Define()))
	assert.Ok(err)

	_, err = ctx.typeDecl.Write([]byte{'\n'})
	assert.Ok(err)
}

func (ctx *ctx) newTempName(name string) string {
	if name == "" {
		name = "_temp"
	}
	ctx.idGenerator++
	return name + strconv.FormatInt(ctx.idGenerator, 10)
}

func (self *ctx) sizeOf(t qbeil.Type) (bits int, align int) {
	return qbeil.SizeOf(self.defaultAlign, t)
}

func flattenRecordAccessPath(expr *parse.RecordAccess) string {
	lhs := expr.Record
	modulePath := []string{}
	for {
		newLhs, ok := lhs.(*parse.RecordAccess)
		if !ok {
			break
		}
		modulePath = append(modulePath, newLhs.Field)
		lhs = newLhs
	}

	var b strings.Builder
	for i := len(modulePath) - 1; i > 0; i-- {
		b.WriteString(modulePath[i])
		b.WriteString(".")
	}
	b.WriteString(modulePath[0])

	return b.String()
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
		}
	}

	return vars
}

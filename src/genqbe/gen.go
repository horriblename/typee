package genqbe

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/can"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/genqbe/qbeil"
	"github.com/horriblename/typee/src/internal/scope"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/simplesub"
	"github.com/horriblename/typee/src/types"
)

// compiler bugs
var ErrCannotCompilePolymorphicType = errors.New("tried to compile a polymorphic type")

// copied from gtype.h, should replace with something less stupid
const G_TYPE_OBJECT int = 20 << 2

type ctx struct {
	module can.ModuleName

	ptrType      qbeil.BaseType
	intType      qbeil.BaseType
	defaultAlign int

	typeDecl       bytes.Buffer
	il             qbeil.Builder
	astToType      map[int]simplesub.TypeScheme
	localTypes     map[string]simplesub.TypeScheme
	allModules     map[can.ModuleName]simplesub.ModuleInfo
	simplified     map[int]types.Type
	statics        map[qbeil.Var]string
	globals        map[string]int // maps global var names to their AST id
	userTypes      map[string]qbeil.AggregateType
	recordTypes    map[int]qbeil.AggregateType
	externs        map[string]struct{}
	vars           scope.ScopedMap[qbeil.Value]
	classInProcess *types.Class

	idGenerator          int64
	generatedTranslation map[types.Type]qbeil.AggregateType
}

func Gen(
	w io.Writer,
	module can.ModuleName,
	modules map[can.ModuleName]simplesub.ModuleInfo,
	ast []parse.Expr,
	typeDefsAst []parse.Expr,
) {
	mainMod := assert.Get(modules, module, "BUG in qbe gen: missing module info of ", module)
	ctx := ctx{
		module:               module,
		ptrType:              qbeil.Long, // TODO: infer ptr & int size + manual options
		intType:              qbeil.Long,
		defaultAlign:         64,
		typeDecl:             bytes.Buffer{},
		il:                   qbeil.Builder{OutFile: w},
		astToType:            mainMod.TypeTree,
		localTypes:           mainMod.Types,
		allModules:           modules,
		simplified:           map[int]types.Type{},
		statics:              map[qbeil.Var]string{},
		globals:              globals(ast),
		userTypes:            map[string]qbeil.AggregateType{},
		recordTypes:          map[int]qbeil.AggregateType{},
		externs:              map[string]struct{}{},
		vars:                 scope.NewScopedMap[qbeil.Value](),
		idGenerator:          0,
		generatedTranslation: map[types.Type]qbeil.AggregateType{},
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

	// struct _GTypeInfo {
	//   /* interface types, classed types, instantiated types */
	//   guint16                class_size;
	//
	//   GBaseInitFunc          base_init;
	//   GBaseFinalizeFunc      base_finalize;
	//
	//   /* interface types, classed types, instantiated types */
	//   GClassInitFunc         class_init;
	//   GClassFinalizeFunc     class_finalize;
	//   gconstpointer          class_data;
	//
	//   /* instantiated types */
	//   guint16                instance_size;
	//   guint16                n_preallocs;
	//   GInstanceInitFunc      instance_init;
	//
	//   /* value handling */
	//   const GTypeValueTable	*value_table;
	// };
	ctx.declareType("GTypeInfo", qbeil.StructType{
		Name: "GTypeInfo",
		Fields: []qbeil.RepeatType{
			qbeil.SingleType(qbeil.HalfWord), // class_size

			qbeil.SingleType(ctx.ptrType),
			qbeil.SingleType(ctx.ptrType),

			qbeil.SingleType(ctx.ptrType),
			qbeil.SingleType(ctx.ptrType),
			qbeil.SingleType(ctx.ptrType),

			qbeil.SingleType(qbeil.HalfWord),
			qbeil.SingleType(qbeil.HalfWord),
			qbeil.SingleType(ctx.ptrType),

			qbeil.SingleType(ctx.ptrType),
		},
	})

	ctx.declareType("GObjectClass", qbeil.StructType{
		Name: "GObjectClass",
		Fields: []qbeil.RepeatType{
			qbeil.SingleType(ctx.ptrType), // GTypeClass, has one field GType

			qbeil.SingleType(ctx.ptrType), // GSList*

			qbeil.SingleType(ctx.ptrType), // function pointer: constructor
			qbeil.SingleType(ctx.ptrType), // function pointer: set_property
			qbeil.SingleType(ctx.ptrType), // function pointer: get_property
			qbeil.SingleType(ctx.ptrType), // function pointer: dispose
			qbeil.SingleType(ctx.ptrType), // function pointer: finalize

			qbeil.SingleType(ctx.ptrType), // function pointer: dispatch_properties_changed
			qbeil.SingleType(ctx.ptrType), // function pointer: notify
			qbeil.SingleType(ctx.ptrType), // function pointer: constructed

			qbeil.SingleType(ctx.ptrType), // gsize
			qbeil.SingleType(ctx.ptrType), // gsize

			qbeil.SingleType(ctx.ptrType), // gpointer
			qbeil.SingleType(ctx.ptrType), // gsize

			{Type: ctx.ptrType, Count: 3}, // padding
		},
	})

	for _, expr := range ast {
		if fn, ok := expr.(*parse.FuncDef); ok && fn.Extern {
			ctx.externs[fn.Name] = struct{}{}
		}
	}

	// generate type definitions first
	for _, expr := range typeDefsAst {
		switch e := expr.(type) {
		case *parse.ObjectTypeDef:
			genClassDef(&ctx, e)
		case *parse.UnionDef:
			genUnionDef(&ctx, e)
		case *parse.EnumDef, *parse.TypeAlias:
		default:
			panic(fmt.Sprintf("unexpected AST node in type defs AST: %#v", expr))
		}
	}

	// generate functions and global vars
	for _, expr := range ast {
		genTopLevel(&ctx, expr)
	}

	ctx.finish()
}

func genTopLevel(ctx *ctx, expr parse.Expr) {
	switch e := expr.(type) {
	case *parse.Import:
	case *parse.Set:
		genGlobalVar(ctx, e)
	case *parse.FuncDef:
		if !e.Extern {
			gen(ctx, expr)
		}
	// type defs are processed earlier
	case *parse.ObjectTypeDef, *parse.UnionDef, *parse.EnumDef, *parse.TypeAlias:

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
	case *parse.SelfLiteral:
		return qbeil.Var{Global: false, Name: "self"}

	case *parse.Symbol:
		if e.Name == "null" {
			return qbeil.IntLiteral{Value: 0}
		}
		if val, ok := ctx.vars.Get(e.Name).Unwrap(); ok {
			return val
		} else if _, ok := ctx.globals[e.Name]; ok {
			return qbeil.Var{Global: true, Name: e.Name}
		}
		panic("BUG: an undefined variable made it's way to code gen phase: " + e.Name)

	case *parse.ExternCall:
		target := ctx.il.TempVar(false)

		argTypes := fun.Map(e.Args, func(arg parse.Expr) types.Type {
			return ctx.simplify(arg.ID())
		})
		args := fun.ZipMap(e.Args, argTypes, func(arg parse.Expr, typ types.Type) qbeil.ABITypedValue {
			return qbeil.ABITypedValue{Type: ctx.toABIType(typ), Value: gen(ctx, arg)}
		})
		funcVar := qbeil.Var{Global: true, Name: e.Symbol.Name}

		retTy := ctx.simplify(e.ID())

		ctx.il.Call(&target, ctx.toABIType(retTy), funcVar, args)

		return target

	case *parse.RecordAccess:
		lhsTy := ctx.simplify(e.Record.ID())
		if lhs, ok := e.Record.(*parse.RecordAccess); ok {
			// TODO: right now we just assume a module path, but we need to handle records too
			module := flattenRecordAccessPath(lhs)
			mangled := mangleName(mangleOpts{
				module: module,
				class:  "",
				name:   e.Field,
			})

			return qbeil.Var{Global: true, Name: mangled}
		}

		switch lhsTy := lhsTy.(type) {
		case *types.Class:
			return genClassAccessByName(ctx, lhsTy.Module, lhsTy.Name, e)
		case *types.Record:
			// TODO: right now we just assume a module path, but we need to handle records too
			// should be unreachable currently
			if lhs, ok := e.Record.(*parse.Symbol); ok {
				mangled := mangleName(mangleOpts{
					module: can.ModuleName(lhs.Name),
					class:  "",
					name:   e.Field,
				})
				return qbeil.Var{Global: true, Name: mangled}
			}
			panic("TODO record access with non-symbol record-typed lhs")
		case *types.Application:
			if len(lhsTy.Params) != 0 {
				panic(fmt.Sprintf("use of polymorphic type on left hand side of %s not yet supported", e.Pretty()))
			}
			lhsTs := ctx.findType(lhsTy.Module, lhsTy.Name)
			switch lhsConcrete := lhsTs.(type) {
			case simplesub.ObjectType:
				return genClassAccessByName(ctx, lhsConcrete.Module, lhsConcrete.Name, e)
			case simplesub.Record:
				ilTy := ctx.toILType(lhsTy)

				structTy, ok := ilTy.(qbeil.StructType)
				assert.True(ok, fmt.Sprintf(
					"BUG: expected %s to be a struct type but is a %T",
					lhsTy.Name, ilTy))
				return genRecordAccess(ctx, e, structTy)
			default:
				panic("TODO Application resulting in unhandled type: " + lhsTs.String())
			}

		default:
			panic(fmt.Sprintf("unexpected types.Type: %#v", lhsTy))
		}

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

	case *parse.Record:
		ilTy := ctx.toILType(ctx.simplify(e.ID()))
		aggTy, ok := ilTy.(qbeil.StructType)
		if !ok {
			panic("Record literal has struct il type")
		}
		structBits, _ := ctx.sizeOf(ilTy)

		rcdPtr := ctx.il.TempVar(false)

		ctx.il.Arithmetic(rcdPtr.IL(), ctx.ptrType, "alloc4", qbeil.IntLiteral{Value: int64(structBits / 8)})

		for _, field := range e.Fields {
			layout := aggTy.Layouts[field.Name]
			offsetBits := layout.OffsetBits

			fieldBits, _ := ctx.sizeOf(layout.Type)
			fieldPtr := ctx.il.TempVar(false)
			ctx.il.Arithmetic(fieldPtr.IL(), ctx.ptrType, "add", rcdPtr, qbeil.IntLiteral{Value: int64(offsetBits / 8)})

			suffix := bitSizeToIntType(fieldBits).IL()
			ctx.il.Command("store"+suffix, gen(ctx, field.Value), fieldPtr)
		}

		return rcdPtr

	case *parse.LetExpr:
		return genLet(ctx, e)
	}

	panic("unimpl gen " + expr.String())
}

func genGlobalVar(ctx *ctx, expr *parse.Set) (val qbeil.Value) {
	name := mangleName(mangleOpts{module: ctx.module, name: expr.Name})
	switch val := expr.Value.(type) {
	case *parse.IntLiteral:
		return ctx.il.Data(
			qbeil.DataDef{
				Linkage: qbeil.Linkage{},
				VarName: name,
				Align:   0,
			},
			ctx.intType,
			qbeil.IntLiteral{
				Value: val.Number,
			})

	case *parse.StrLiteral:
		return ctx.il.StrData(qbeil.DataDef{
			Linkage: qbeil.Linkage{},
			VarName: name,
			Align:   0,
		}, val.Content)

	case *parse.FloatLiteral:
		return ctx.il.Data(
			qbeil.DataDef{
				Linkage: qbeil.Linkage{},
				VarName: name,
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
				VarName: name,
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

func (ctx *ctx) findType(module can.ModuleName, name string) simplesub.TypeScheme {
	if module == "" {
		return assert.Get(ctx.localTypes, name, "BUG: encountered missing local type during type gen: ", name)
	}
	mod := assert.Get(ctx.allModules, module, "BUG: encountered unresolved module during type gen:", module)
	return assert.Get(mod.Types, name, "BUG: encountered unresolved imported type during type gen:", name, "of", module)
}

// class should be empty string for non-methods
func genFunc(ctx *ctx, class string, expr *parse.FuncDef) (val qbeil.Value) {
	defer func() {
		if e := recover(); e != nil {
			if class != "" {
				class = class + "#"
			}
			panic(fmt.Sprintf("in function %s%s: %v", class, expr.Name, e))
		}
	}()

	ctx.vars.NewScope()
	defer ctx.vars.PopScope()

	friendlyName := fmt.Sprintf("%s.%s", class, expr.Name)
	realArgTys := []types.Type{}
	var retTy types.Type
	{
		funcTyp, ok := ctx.simplify(expr.ID()).(*types.Func)
		assert.True(ok, "generate function code: type of ", friendlyName, " is not function")
		retTy = funcTyp.Ret
		if funcTyp.Method {
			assert.Eq(len(expr.Args), len(funcTyp.Args)+1, "gen BUG: arg count from AST != len(funcTyp.Args)")
			assert.True(ctx.classInProcess != nil, "gen BUG: generating method ", friendlyName, ": classInProcess is nil")
			realArgTys = append([]types.Type{ctx.classInProcess}, funcTyp.Args...)
		} else {
			assert.Eq(len(expr.Args), len(funcTyp.Args), "gen BUG: arg count from AST != len(funcTyp.Args)")
			realArgTys = funcTyp.Args
		}
	}
	assert.GreaterThan(len(expr.Body), 0, "function", friendlyName, "has empty body")

	mangled := expr.Name
	if expr.Name != "main" {
		mangled = mangleName(mangleOpts{module: ctx.module, class: class, name: expr.Name})
	}

	thisFunc := qbeil.Var{Global: true, Name: mangled}
	argTyps := make([]qbeil.TypedVar, 0, len(realArgTys))

	for i, argTyp := range realArgTys {
		val := qbeil.Var{Global: false, Name: expr.Args[i]}
		argTyps = append(argTyps, qbeil.NewTypedVar(
			ctx.toABIType(argTyp),
			val,
		))
		ctx.vars.Insert(expr.Args[i], val)
	}

	// TODO: don't export all symbols
	linkage := qbeil.Linkage{Type: qbeil.Export}
	retTyp := ctx.toABIType(retTy)
	if expr.Name == "main" && class == "" {
		linkage.Type = qbeil.Export
		retTyp = qbeil.Word
	}

	assert.Ok(ctx.il.Func(linkage, retTyp, thisFunc.IL(), argTyps))

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
			[]qbeil.ABITypedValue{{
				Type:  ctx.intType,
				Value: qbeil.IntLiteral{Value: int64(bits / 8)}},
			},
		)

		return val

	case *parse.MethodAccess:
		ty := ctx.simplify(callee.Obj.ID())
		var class string
		switch t := ty.(type) {
		case *types.Class:
			class = t.Name
		case *types.Application:
			class = t.Name
			assert.Eq(len(t.Params), 0, "parameterized function call not supported yet")
		}
		assert.Neq(class, "", "unnamed class not yet supported")
		return genCallWithFuncName(ctx, ctx.module, class, callee.Method, expr)

	case *parse.RecordAccess:
		// TODO: currently only module access supported
		var modulePath can.ModuleName
		switch lhs := callee.Record.(type) {
		case *parse.Symbol:
			modulePath = can.ModuleName(lhs.Name)
		case *parse.RecordAccess:
			modulePath = flattenRecordAccessPath(lhs)
		default:
			panic(fmt.Sprintf("unexpected lhs of record accessor %T in: %s", callee.Record, callee))
		}

		return genCallWithFuncName(ctx, modulePath, "", callee.Field, expr)

	case *parse.Symbol:
		return genCallWithFuncName(ctx, ctx.module, "", callee.Name, expr)

	default:
		panic("unimpl: genCall for callee of the form " + expr.Pretty())
	}
}

func genCallWithFuncName(ctx *ctx, module can.ModuleName, class string, fnName string, expr *parse.Form) qbeil.Value {
	mangled := fnName
	if !mapHas(ctx.externs, fnName) /* FIXME: might get shadowed */ {
		mangled = mangleName(mangleOpts{module: module, class: class, name: fnName})
	}

	fnFriendlyName := fnName
	if class != "" {
		fnFriendlyName = fmt.Sprintf("%s.%s", class, fnName)
	}
	callee := expr.Children[0]

	switch fnName {
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
		ctx.il.Call(nil, qbeil.Long, qbeil.Var{Global: true, Name: "exit"}, []qbeil.ABITypedValue{
			{Type: ctx.intType, Value: exitCode},
		})
		return qbeil.IntLiteral{Value: 0} // TODO: there's probably a better way
	case "print":
		assert.Eq(len(expr.Children), 2, `wrong arg count for "print"`)
		arg := gen(ctx, expr.Children[1])
		target := ctx.il.TempVar(false)
		ctx.il.Call(&target, ctx.ptrType,
			qbeil.Var{Global: true, Name: "print"},
			[]qbeil.ABITypedValue{{
				Type:  ctx.toABIType(&types.String{}),
				Value: arg,
			}})

		return target

	case "stackAlloc":
		assert.Eq(len(expr.Children), 1, `wrong arg count for "stackAlloc`)

		ilTy := ctx.toILType(ctx.simplify(expr.ID()))
		bits, _ := ctx.sizeOf(ilTy)

		dataPtr := ctx.il.TempVar(false)
		ctx.il.Arithmetic(dataPtr.IL(), ctx.ptrType, "alloc4", qbeil.IntLiteral{Value: int64(bits / 8)})
		return dataPtr

	case "deref":
		assert.Eq(len(expr.Children), 2, `wrong arg count for "deref`)

		ty := ctx.simplify(expr.Children[1].ID())
		refTy, ok := ty.(*types.Ref)
		if !ok {
			panic(fmt.Sprintf("during codegen: deref expects a Ref type, got: %v", ty))
		}

		dataTy, ok := refTy.Content.Unwrap()
		if !ok {
			panic("during codegen: Opaque references cannot be deref'd")
		}

		ref := gen(ctx, expr.Children[1])
		ilTy := ctx.toILType(dataTy)
		bits, _ := ctx.sizeOf(ilTy)
		stackPtr := ctx.il.TempVar(false)

		ctx.il.Arithmetic(stackPtr.IL(), ctx.ptrType, "alloc4", qbeil.IntLiteral{Value: int64(bits / 8)})

		ctx.il.Command("blit", ref, stackPtr, qbeil.IntLiteral{Value: int64(bits / 8)})

		return stackPtr

	default:
		// TODO: local functions
		fn := ctx.simplify(callee.ID())

		funcSig, ok := fn.(*types.Func)
		// for methods, the class is prepended
		var realArgTys []types.Type
		assert.True(ok, "tried to call non-function top-level:", fnFriendlyName, "of type", fmt.Sprintf("%#v", fn))
		if meth, ok := expr.Children[0].(*parse.MethodAccess); ok {
			assert.Eq(len(expr.Children), len(funcSig.Args)+1, meth.Pretty(), ": method argument count does not match signature")
			objTy := ctx.simplify(meth.Obj.ID())
			realArgTys = append([]types.Type{objTy}, funcSig.Args...)
		} else {
			assert.Eq(len(expr.Children), len(funcSig.Args)+1, fnFriendlyName, ": function argument count does not match signature")
			realArgTys = funcSig.Args
		}
		target := ctx.il.TempVar(false)

		var args []qbeil.ABITypedValue
		if meth, ok := expr.Children[0].(*parse.MethodAccess); ok {
			args = []qbeil.ABITypedValue{{
				Type:  ctx.toABIType(realArgTys[0]),
				Value: gen(ctx, meth.Obj),
			}}
		}
		for arg, typ := range fun.ZipSlices(expr.Children[1:], realArgTys) {
			args = append(args, qbeil.ABITypedValue{Type: ctx.toABIType(typ), Value: gen(ctx, arg)})
		}
		funcVar := qbeil.Var{Global: true, Name: mangled}

		ctx.il.Call(&target, ctx.toABIType(funcSig.Ret), funcVar, args)

		return target
	}
}

func genLet(ctx *ctx, expr *parse.LetExpr) qbeil.Value {
	ctx.vars.NewScope()
	defer ctx.vars.PopScope()

	for _, ass := range expr.Assignments {
		rhsVal := gen(ctx, ass.Value)
		ctx.vars.Insert(ass.Var, rhsVal)
	}

	return gen(ctx, expr.Body)
}

func genClassDef(ctx *ctx, e *parse.ObjectTypeDef) {

	// TODO: support generics
	ct := ctx.simplify(e.ID())
	classTy, ok := ct.(*types.Class)
	if !ok {
		panic(fmt.Sprintf("compiler bug: class definition yields non-class type %#v", ct))
	}

	class := ctx.classDefIL(classTy, e)
	ctx.declareType(e.Name, class)

	// TODO: define GObjectClass
	parentClass := assert.Get(ctx.userTypes, "GObjectClass", "undefined parent class type?")
	if len(e.Supers) > 0 {
		// TODO: what if all are interfaces
		// TODO: generate imported types
		parentClass = ctx.userTypes[e.Supers[0].String()+"Class"]
	}

	// TODO: name collision?
	classType := qbeil.StructType{
		Name: e.Name + "Class",
		Fields: []qbeil.RepeatType{
			// parent_class
			qbeil.SingleType(parentClass),
		},
	}
	ctx.declareType(e.Name+"Class", classType)

	private := qbeil.StructType{
		Name: e.Name + "Private",
		// TODO: handle privates
		Fields: []qbeil.RepeatType{},
	}
	ctx.declareType(e.Name+"Private", private)

	genClassBoilerplate(ctx, e.Name, class, classType, private)
}

// generates functions that take care of initialization
// e.g. function to retrieve the GType of the class
func genClassBoilerplate(
	ctx *ctx,
	className string,
	class qbeil.AggregateType,
	classType qbeil.AggregateType,
	private qbeil.AggregateType,
) {
	classBits, _ := ctx.sizeOf(class)
	classTypeBits, _ := ctx.sizeOf(classType)
	privateBits, _ := ctx.sizeOf(private)

	typeNameVar := ctx.il.StrData(qbeil.DataDef{
		Linkage: qbeil.Linkage{},
		VarName: mangleName(mangleOpts{
			module: ctx.module,
			class:  className,
			name:   "class_name",
		}),
	}, className)

	// TODO: name collision?
	typeIdVarOnce := ctx.il.Data(
		qbeil.DataDef{
			Linkage: qbeil.Linkage{},
			VarName: mangleName(mangleOpts{
				module: ctx.module,
				class:  className,
				name:   "_type_id__once",
			}),
			Align: 0,
		},
		ctx.ptrType, // TODO: is this correct? gsize == guintptr??
		qbeil.IntLiteral{Value: 0},
	)

	typeIdTempVar := ctx.il.TempVar(false)

	typeInfoName := mangleName(mangleOpts{
		module: ctx.module,
		class:  className,
		name:   "g_define_type_info",
	})
	typeInfoConst := ctx.il.CompositeData(
		qbeil.DataDef{
			Linkage: qbeil.Linkage{},
			VarName: typeInfoName,
			Align:   0,
		},
		qbeil.DataItems([]qbeil.TypedDataItem{
			{Type: qbeil.HalfWord, Value: qbeil.IntLiteral{Value: int64(classTypeBits / 8)}},

			// padding TODO: pad automatically
			{Type: qbeil.HalfWord, Value: qbeil.IntLiteral{Value: 0}},
			{Type: qbeil.Word, Value: qbeil.IntLiteral{Value: 0}},

			{Type: ctx.ptrType, Value: qbeil.IntLiteral{Value: 0}},
			{Type: ctx.ptrType, Value: qbeil.IntLiteral{Value: 0}},

			// should be (type)_class_init
			{Type: ctx.ptrType, Value: qbeil.IntLiteral{Value: 0}},
			{Type: ctx.ptrType, Value: qbeil.IntLiteral{Value: 0}},
			{Type: ctx.ptrType, Value: qbeil.IntLiteral{Value: 0}},

			{Type: qbeil.HalfWord, Value: qbeil.IntLiteral{Value: int64(classBits / 8)}},
			{Type: qbeil.HalfWord, Value: qbeil.IntLiteral{Value: 0}},

			// padding TODO: pad automatically
			{Type: qbeil.Word, Value: qbeil.IntLiteral{Value: 0}},

			// should be (type)_instance_init
			{Type: ctx.ptrType, Value: qbeil.IntLiteral{Value: 0}},

			{Type: qbeil.HalfWord, Value: qbeil.IntLiteral{Value: 0}},
		}),
	)

	// get_type function

	ctx.il.Func(
		qbeil.Linkage{Type: qbeil.Export},
		ctx.ptrType, /* GType */
		"$"+mangleName(mangleOpts{
			module: ctx.module,
			class:  className,
			name:   "get_type",
		}),
		[]qbeil.TypedVar{},
	)

	enterResultVar := ctx.il.TempVar(false)
	ctx.il.Call(
		&enterResultVar, // FIXME: boolean?
		ctx.intType,
		qbeil.Var{Global: true, Name: "g_once_init_enter"},
		[]qbeil.ABITypedValue{{Type: ctx.ptrType, Value: typeIdVarOnce}},
	)

	thenLabel := ctx.il.TempLabel("then_")
	elseLabel := ctx.il.TempLabel("else_")
	ctx.il.Jnz(enterResultVar, thenLabel, elseLabel)

	ctx.il.InsertLabel(thenLabel)

	ctx.il.Call(
		&typeIdTempVar,
		// return type is GType
		ctx.ptrType,
		qbeil.Var{Global: true, Name: "g_type_register_static"},
		[]qbeil.ABITypedValue{
			{Type: ctx.ptrType /* GType */, Value: qbeil.IntLiteral{Value: int64(G_TYPE_OBJECT)}},
			{Type: ctx.ptrType, Value: typeNameVar},
			{Type: ctx.ptrType, Value: typeInfoConst},
			// TODO: this is an enum GTypeFlags, idk what type it should be
			{Type: ctx.intType, Value: qbeil.IntLiteral{Value: 0}},
		},
	)

	if privateBits > 0 {
		privateOffset := ctx.il.Data(
			qbeil.DataDef{
				Linkage: qbeil.Linkage{},
				VarName: className + "_private_offset",
			},
			// gint
			ctx.intType,
			qbeil.IntLiteral{Value: 0},
		)

		privateOffsetTemp := ctx.il.TempVar(false)
		ctx.il.Call(
			&privateOffsetTemp,
			ctx.intType,
			qbeil.Var{Global: true, Name: "g_type_add_instance_private"},
			[]qbeil.ABITypedValue{
				{Type: ctx.ptrType /* GType */, Value: typeIdVarOnce},
				{Type: ctx.ptrType /* size_t */, Value: qbeil.IntLiteral{Value: int64(privateBits / 8)}},
			},
		)

		ctx.il.Command("store"+ctx.ptrType.IL(), privateOffsetTemp, privateOffset)
	}

	ctx.il.Call(nil, nil,
		qbeil.Var{Global: true, Name: "g_once_init_leave"},
		[]qbeil.ABITypedValue{
			{Type: ctx.ptrType, Value: typeIdVarOnce},
			{Type: ctx.ptrType, Value: typeIdTempVar},
		},
	)

	ctx.il.InsertLabel(elseLabel)
	ctx.il.Ret(typeIdVarOnce)

	ctx.il.EndFunc()
}

func genUnionDef(ctx *ctx, e *parse.UnionDef) {
	st := ctx.simplify(e.ID())
	unionTy, ok := st.(*types.Union)
	if !ok {
		panic(fmt.Sprintf("compiler bug: union definition yields non-class type %#v", st))
	}

	ctx.declareType(e.Name, ctx.unionDefIL(unionTy, e))
}

func genClassAccessByName(ctx *ctx, module can.ModuleName, class string, expr *parse.RecordAccess) qbeil.Value {
	assert.Neq(class, "", "unnamed class in class access not yet supported")
	assert.Eq(module, "", "imported class not yet supported")

	ct := ctx.userTypes[class]
	classTy, ok := ct.(qbeil.StructType)
	assert.True(ok, "codegen: record access on non-struct type (type checker bug?)", ct, "from expr: ", expr)

	fieldLayout, ok := classTy.Layouts[expr.Field]
	if !ok {
		panic(fmt.Sprintf("typer bug: tried to use a non-existent class field %s.%s", class, expr.Field))
	}

	// TODO: once we support structs, we can't just pass this around (can we?)
	bt, ok := fieldLayout.Type.(qbeil.BaseType)
	if !ok {
		panic("unreachable: embedded struct types are currently invalid in classes")
	}

	// TODO: 32-bit system
	addr := ctx.il.TempVar(false)
	ctx.il.Arithmetic(addr.IL(), ctx.ptrType, "add",
		gen(ctx, expr.Record),
		qbeil.IntLiteral{Value: int64(fieldLayout.OffsetBits)},
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
}

func genRecordAccess(ctx *ctx, expr *parse.RecordAccess, rcdTy qbeil.StructType) qbeil.Value {
	fieldLayout, ok := rcdTy.Layouts[expr.Field]
	if !ok {
		panic(fmt.Sprintf("typer bug: uncaught use of non-existent record field %s", expr.Field))
	}

	bt, ok := fieldLayout.Type.(qbeil.BaseType)
	if !ok {
		panic("unreachable: record field with non-base types currently not allowed")
	}

	// TODO: 32-bit system
	addr := ctx.il.TempVar(false)
	ctx.il.Arithmetic(addr.IL(), ctx.ptrType, "add",
		gen(ctx, expr.Record),
		qbeil.IntLiteral{Value: int64(fieldLayout.OffsetBits)},
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

	io.Copy(ctx.il.OutFile, &ctx.il.Buf)
}

func (ctx *ctx) toILType(typ types.Type) qbeil.Type {
	switch t := typ.(type) {
	case *types.Application:
		mod := ctx.localTypes
		if t.Module != ctx.module {
			m, ok := ctx.allModules[t.Module]
			assert.True(ok, "BUG: unresolved import still in code gen phase: ", t.Module)

			mod = m.Types
		}

		ty, ok := mod[t.Name]
		assert.True(ok, "BUG: unresolved type still in code gen phase, module:", t.Module, ", type:", t.Name)

		if pt, ok := ty.(simplesub.PolymorphicType); ok {
			panic(fmt.Sprintf("BUG polymorphic type should not be toILType'd? %v", pt))
		}

		st := assert.Cast[simplesub.SimpleType](ty, "already checked for PolymorphicType")
		return ctx.toILType(ctx.simplifyType(st))

	case *types.Int:
		switch t.BitSize {
		case 8:
			return qbeil.Byte
		case 16:
			return qbeil.HalfWord
		case 32:
			return qbeil.Word
		case 64:
			return qbeil.Long
		}
		panic(fmt.Sprint("illegal integer bit size:", t.BitSize))
	case *types.Float:
		return qbeil.Double
	case *types.Bool:
		return ctx.intType
	case *types.String:
		return ctx.userTypes["Str"]
	case *types.Class:
		return ctx.ptrType
	case *types.Record:
		return ctx.recordToILType(t)

	case *types.Ref:
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

func (ctx *ctx) toABIType(typ types.Type) qbeil.ABIType {
	switch t := typ.(type) {
	case *types.Int:
		switch t.BitSize {
		case 8:
			if t.Signed {
				return qbeil.SignedByte
			} else {
				return qbeil.UnsignedByte
			}
		case 16:
			if t.Signed {
				return qbeil.SignedHalf
			} else {
				return qbeil.UnsignedByte
			}
		case 32:
			return qbeil.Word
		case 64:
			return qbeil.Long
		default:
			panic(fmt.Sprintf("illegal bitsize in integer: %d", t.BitSize))
		}
	default:
		// FIXME: fix qbeil types
		return ctx.toILType(typ).(qbeil.ABIType)
	}
}

func (ctx *ctx) recordToILType(t *types.Record) qbeil.StructType {
	if il, ok := ctx.generatedTranslation[t]; ok {
		return il.(qbeil.StructType)
	}

	fields := make([]qbeil.RepeatType, 0, len(t.Fields))
	layouts := map[string]qbeil.FieldLayout{}
	offset := 0

	for name, fieldTy := range t.Fields {
		ilTy := ctx.toILType(fieldTy)
		bits, _ := ctx.sizeOf(ilTy) // TODO: align
		fields = append(fields, qbeil.SingleType(ilTy))
		layouts[name] = qbeil.FieldLayout{
			Type:       ilTy,
			OffsetBits: offset,
		}
		offset += bits
	}

	name := ctx.newTempName("Record_")
	ilTyp := qbeil.StructType{
		Align:   0,
		Layouts: layouts,
		Name:    name,
		Fields:  fields,
	}

	ctx.declareType(name, ilTyp)
	ctx.generatedTranslation[t] = ilTyp
	return ilTyp
}

// like [ctx.toILType] but converts class type to its full [qbeil.StructType] instead of a pointer type
func (ctx *ctx) classDefIL(t *types.Class, e *parse.ObjectTypeDef) qbeil.AggregateType {
	if t.Name == "" {
		// FIXME: generic support
		panic("unnamed classes should be illegal at codegen")
	}

	// FIXME: why did I put this here?? there's no way a userType already exists during class definition right?
	if ut, ok := ctx.userTypes[t.Name]; ok {
		return ut
	}

	ctx.classInProcess = t
	defer func() { ctx.classInProcess = nil }()

	var classParent qbeil.AggregateType
	if len(e.Supers) != 0 {
		// TODO: assert super is class or something
		classParent = assert.Get(ctx.userTypes, e.Supers[0].String(), "super type of ", e.Name, ":", e.Supers[0], "not found?")
	} else if !e.Base {
		classParent = assert.Get(ctx.userTypes, "GObject", "type Object not defined?")
	}

	fields := []qbeil.RepeatType{}
	pubOffset := 0
	if classParent != nil {
		fields = append(fields, qbeil.SingleType(classParent))
		bits, _ := ctx.sizeOf(classParent)
		pubOffset += bits / 8
	}
	fields = append(fields, qbeil.SingleType(ctx.ptrType)) // private pointer
	bits, _ := ctx.sizeOf(ctx.ptrType)
	pubOffset += bits / 8

	layouts := map[string]qbeil.FieldLayout{}

	for name, field := range t.Fields {
		if field.Access == parse.AccessPublic || field.Access == parse.AccessProtected {
			ilTy := ctx.toILType(field.Type)
			bits, _ := ctx.sizeOf(ilTy) // TODO: align
			fields = append(fields, qbeil.SingleType(ilTy))
			layouts[name] = qbeil.FieldLayout{
				OffsetBits: pubOffset,
				Type:       ilTy,
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

	st, ok := ctx.astToType[exprID].(simplesub.SimpleType)
	if !ok {
		pt, ok := ctx.astToType[exprID].(simplesub.PolymorphicType)
		// HACK: temp workaround for class and method types
		if !ok {
			panic(fmt.Sprintf("unreachable or nil TypeScheme at expr ID %d: %v", exprID, ctx.astToType[exprID]))
		}
		st = pt.Body
	}
	t1 := simplesub.SimplifyType(st)
	t2 := simplesub.CoalesceType(t1)
	ctx.simplified[exprID] = t2
	return t2
}

func (ctx *ctx) simplifyType(ty simplesub.SimpleType) types.Type {
	t1 := simplesub.SimplifyType(ty)
	t2 := simplesub.CoalesceType(t1)

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

func (self *ctx) sizeOf(t qbeil.Type) (bits int, alignBits int) {
	return qbeil.SizeOf(self.defaultAlign, t)
}

func flattenRecordAccessPath(expr *parse.RecordAccess) can.ModuleName {
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

	return can.ModuleName(b.String())
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

func bitSizeToIntType(bits int) qbeil.Type {
	switch bits {
	case 8:
		return qbeil.Byte
	case 16:
		return qbeil.HalfWord
	case 32:
		return qbeil.Word
	case 64:
		return qbeil.Long
	}
	panic(fmt.Sprintf("invalid bit size for int type: %d", bits))
}

func mapHas[K comparable, V any](m map[K]V, key K) bool {
	_, ok := m[key]
	return ok
}

package genqbe

import (
	"bytes"
	"cmp"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/can"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/genqbe/qbeil"
	"github.com/horriblename/typee/src/internal/ordered"
	"github.com/horriblename/typee/src/internal/scope"
	"github.com/horriblename/typee/src/opt"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/simplesub"
	"github.com/horriblename/typee/src/types"
)

// compiler bugs
var ErrCannotCompilePolymorphicType = errors.New("tried to compile a polymorphic type")

// copied from gtype.h, should replace with something less stupid
const G_TYPE_OBJECT int = 20 << 2
const G_TYPE_INTERFACE int = 2 << 2

type exprError struct {
	expr parse.Expr
	err  any
}

func (self exprError) Error() string {
	return fmt.Sprintf("in expression %s: %v", self.expr.Pretty(), self.err)
}

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
	imports        map[string]can.ModuleName
	vars           scope.ScopedMap[varInfo]
	classInProcess *types.Class

	funcInProcess       string
	unprocessedClosures []closure

	idGenerator          int64
	generatedTranslation map[types.Type]qbeil.AggregateType
}

type varInfo struct {
	// unique IL name (just make sure this doesn't clash with other local vars)
	uname string

	val qbeil.Value
	typ types.Type

	// currently unused
	lastUse opt.Option[parse.ID]
}

type closure struct {
	name string
	expr *parse.Fn

	captureData  *qbeil.StructType
	captureTypes map[string]types.Type
}

func Gen(
	w io.Writer,
	module can.ModuleName,
	modules map[can.ModuleName]simplesub.ModuleInfo,
	ast []parse.Expr,
	typeDefsAst []parse.Expr,
) error {
	mainMod := assert.Get(modules, module, "BUG in qbe gen: missing module info of ", module)
	ctx := ctx{
		module:               module,
		ptrType:              qbeil.Long, // TODO: infer ptr & int size + manual options
		intType:              qbeil.Long, // TODO: wtf should I put here
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
		imports:              map[string]can.ModuleName{},
		vars:                 scope.NewScopedMap[varInfo](),
		idGenerator:          0,
		generatedTranslation: map[types.Type]qbeil.AggregateType{},
	}
	ptrSizeBits, _ := ctx.sizeOf(ctx.ptrType)

	ctx.declareType("Str", qbeil.StructType{
		Align: 0,
		Name:  "Str",
		Layouts: map[string]qbeil.FieldLayout{
			"data": {Type: ctx.ptrType, OffsetBits: 0},
			"size": {Type: ctx.ptrType, OffsetBits: ptrSizeBits},
		},
		Fields: []qbeil.RepeatType{
			qbeil.SingleType(ctx.ptrType),
			qbeil.SingleType(ctx.ptrType),
		},
	})

	ctx.declareType("List", qbeil.StructType{
		Align:   0,
		Name:    "List",
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
		Name: "GObject",
		Layouts: map[string]qbeil.FieldLayout{
			"g_class": {
				Type:       ctx.ptrType,
				OffsetBits: 0,
			},
		},
		Fields: []qbeil.RepeatType{
			qbeil.SingleType(ctx.ptrType),
			qbeil.SingleType(ctx.intType),
			qbeil.SingleType(ctx.ptrType),
		},
		Align: 0,
	})

	// struct GTypeClass {
	//   GType g_type;
	// }
	ctx.declareType("GTypeClass", qbeil.StructType{
		Name: "GTypeClass",
		Layouts: map[string]qbeil.FieldLayout{
			"g_type": {
				Type: ctx.ptrType,
			},
		},
		Fields: []qbeil.RepeatType{
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

	ctx.declareType("GInterfaceInfo", qbeil.StructType{
		Name: "GInterfaceInfo",
		Fields: []qbeil.RepeatType{
			qbeil.SingleType(ctx.ptrType), // init
			qbeil.SingleType(ctx.ptrType), // finalize
			qbeil.SingleType(ctx.ptrType), // data
		},
	})

	ctx.declareType("ClosureComponents", qbeil.StructType{
		Align: 0,
		Name:  "ClosureComponents",
		Fields: []qbeil.RepeatType{{
			Type:  ctx.ptrType,
			Count: 3,
		}},
		Layouts: map[string]qbeil.FieldLayout{
			"func":    {Type: ctx.ptrType, OffsetBits: 0},
			"data":    {Type: ctx.ptrType, OffsetBits: ptrSizeBits},
			"cleanup": {Type: ctx.ptrType, OffsetBits: ptrSizeBits * 2},
		},
	})

	// partial structure of Box<T>, only covering the ref count
	// Box should be passed around as pointers so this should be fine?
	ctx.declareType("BoxPartial", qbeil.StructType{
		Name: "BoxPartial",
		Fields: []qbeil.RepeatType{
			qbeil.SingleType(ctx.ptrType),
		},
		Align: 0,
		Layouts: map[string]qbeil.FieldLayout{
			"refCount": {
				Type:       ctx.ptrType,
				OffsetBits: 0,
			},
			"data": {
				Type: ctx.ptrType,
			},
		},
	})

	// Map imports and write extern functions
	for _, expr := range ast {
		switch e := expr.(type) {
		case *parse.FuncDef:
			if e.Extern {
				ctx.externs[e.Name] = struct{}{}
			}
		case *parse.Import:
			// I really should find a way to not do this.
			// Canonicalization will most likely help.
			mod := can.ModuleName(strings.Join(e.Module, "."))
			alias := e.Module[len(e.Module)-1]
			ctx.imports[alias] = mod
		}
	}

	// generate type definitions first
	for _, expr := range typeDefsAst {
		switch e := expr.(type) {
		case *parse.ObjectTypeDef:
			if e.Kind == parse.Class {
				genClassDef(&ctx, e)
			} else {
				genInterfaceDef(&ctx, e)
			}
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
		closure, ok := popSlice(&ctx.unprocessedClosures).Unwrap()
		for ; ok; closure, ok = popSlice(&ctx.unprocessedClosures).Unwrap() {
			// reassign captured values to their original names
			def := parse.FuncDef{
				Id:        closure.expr.ID(),
				Name:      closure.name,
				Signature: closure.expr.Signature,
				Args:      closure.expr.Args,
				Body:      []parse.Expr{closure.expr.Body},
				Extern:    false,
				Synth:     true,
			}
			ctx.funcInProcess = closure.name
			genFunc(&ctx, "", &def, true, closure.captureData, closure.captureTypes)
			ctx.funcInProcess = ""
		}
	}

	return ctx.finish()
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
	defer func() {
		if e := recover(); e == nil {
			return
		} else if ee, ok := e.(exprError); ok {
			panic(ee)
		} else {
			panic(exprError{expr: expr, err: e})
		}
	}()
	switch e := expr.(type) {
	case *parse.IntLiteral:
		return qbeil.IntLiteral{Value: e.Number}
	case *parse.FuncDef:
		ctx.funcInProcess = e.Name
		defer func() { ctx.funcInProcess = "" }()
		return genFunc(ctx, "", e, false, nil, nil)
	case *parse.Fn:
		return genClosure(ctx, e)
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
	case *parse.BoolLiteral:
		if e.Value {
			return qbeil.IntLiteral{Value: 1}
		}
		return qbeil.IntLiteral{Value: 0}
	case *parse.SelfLiteral:
		return qbeil.Var{Global: false, Name: "self"}

	case *parse.Symbol:
		if e.Name == "null" {
			return qbeil.IntLiteral{Value: 0}
		}
		if val, ok := ctx.vars.Get(e.Name).Unwrap(); ok {
			return val.val
		} else if _, ok := ctx.globals[e.Name]; ok {
			mangled := mangleName(mangleOpts{
				module: ctx.module,
				class:  "",
				name:   e.Name,
			})
			funcVar := qbeil.Var{Global: true, Name: mangled}
			if _, ok := ctx.resolveTypeApplications(ctx.simplify(e.ID())).(*types.Func); ok {
				// FIXME: this should be handled as a coercion somewhere else probably?
				// I also need to separate raw function pointer types and closures
				ilTy := assert.Get(ctx.userTypes, "ClosureComponents",
					"BUG codegen: IL type ClosureComponents not defined?")
				sTy := assert.Cast[qbeil.StructType](ilTy,
					"BUG codegen: IL Type of ClosureComponents is not a StructType?")
				// HACK: this is not correct, the function will be passed an
				// extra data argument (which is null here) when called as a
				// closure; but this works under SysV ABI so uh, I'll fix it
				// later
				return genRecordLiteral(ctx, sTy, false, []recordAssignment{
					{name: "func", typ: ctx.ptrType, value: funcVar},
					{name: "data", typ: ctx.ptrType, value: qbeil.IntLiteral{Value: 0}},
					{name: "cleanup", typ: ctx.ptrType, value: qbeil.IntLiteral{Value: 0}},
				})
			} else {
				return funcVar
			}
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
		return genDotAccessOnAny(ctx, e)

	case *parse.StrLiteral:
		dataGlobal := ctx.il.TempVar(true)
		ctx.statics[dataGlobal] = fmt.Sprintf(`{b "%s"}`, e.Content)

		// the Str struct on stack
		strPtr := ctx.il.TempVar(false)
		strSizeBits, _ := ctx.sizeOf(assert.Get(ctx.userTypes, "Str",
			"genqbe: BUG missing definition of Str"))
		ptrSizeBits, _ := ctx.sizeOf(ctx.ptrType)

		ctx.il.Arithmetic(strPtr.IL(), qbeil.Long, "alloc4", qbeil.IntLiteral{Value: int64(strSizeBits / 8)})
		ctx.il.Command("storel", dataGlobal, strPtr)

		lenPtr := ctx.il.TempVar(false)
		ctx.il.Arithmetic(lenPtr.IL(), qbeil.Long, "add", strPtr, qbeil.IntLiteral{Value: int64(ptrSizeBits / 8)})
		ctx.il.Command("storel", qbeil.IntLiteral{Value: int64(len(e.Content))}, lenPtr)

		return strPtr

	case *parse.Record:
		if len(e.Fields) == 0 {
			return qbeil.IntLiteral{Value: 0}
		}
		ilTy := ctx.toILType(ctx.simplify(e.ID()))
		aggTy, ok := ilTy.(qbeil.StructType)
		if !ok {
			panic("BUG: Record literal is not a struct IL type but a " + aggTy.IL())
		}
		ass := fun.Map(e.Fields, func(field parse.RecordField) recordAssignment {
			return recordAssignment{
				name:  field.Name,
				typ:   ctx.toILType(ctx.simplify(field.Value.ID())),
				value: gen(ctx, field.Value),
			}
		})
		return genRecordLiteral(ctx, aggTy, false, ass)

	case *parse.LetExpr:
		return genLet(ctx, e)

	case *parse.IfExpr:
		return genIf(ctx, e)
	case *parse.ArrayLiteral:
		list := ctx.il.TempNamedVar(false, "list_ctor")
		// I don't think this can be an Application?
		listTy := assert.Cast[*types.Slice](ctx.simplify(e.ID()))
		contentTy := listTy.Type
		contentIlTy := ctx.toILType(contentTy)
		listStruct := assert.Get(ctx.userTypes, "List",
			"BUG codegen: List struct type not defined?")
		bits, _ := ctx.sizeOf(ctx.toILType(contentTy))
		size := bitsToBytesRoundedUp(bits)
		sizeVal := qbeil.IntLiteral{Value: int64(size)}
		preallocateFuncVar := qbeil.Var{
			Global: true,
			Name:   "preallocateList",
		}
		ctx.il.Call(&list, listStruct, preallocateFuncVar, []qbeil.ABITypedValue{
			{Type: qbeil.Long, Value: qbeil.IntLiteral{Value: int64(len(e.Elements))}},
			{Type: qbeil.Long, Value: sizeVal},
		})

		appendFuncVar := qbeil.Var{Global: true, Name: "listAppend"}

		// append items
		for _, item := range e.Elements {
			newList := ctx.il.TempNamedVar(false, "list_ctor")
			itemVal := gen(ctx, item)
			var itemRef qbeil.Value
			switch contentIlTy.(type) {
			case qbeil.BaseType:
				itemRef = ctx.il.TempNamedVar(false, "list_item_ref")
				ctx.il.Arithmetic(itemRef.IL(), ctx.ptrType, "alloc4", sizeVal)
				ctx.il.Command("store"+ctx.ptrType.IL(), itemVal, itemRef)
			case qbeil.ExtraType:
				panic("codegen: ExtraType in array literal not yet supported")
			case qbeil.StructType, *qbeil.UnionType:
				// is already a pointer, pass it to the function directly
				itemRef = itemVal
			default:
				panic(fmt.Sprintf("unexpected qbeil.Type: %#v", contentIlTy))
			}
			ctx.il.Call(&newList, listStruct, appendFuncVar, []qbeil.ABITypedValue{
				{Type: listStruct, Value: list},
				{Type: qbeil.Long, Value: sizeVal},
				{Type: ctx.ptrType, Value: itemRef},
			})
			list = newList
		}

		return list
	case *parse.TaggedExpr:
		// TODO: I'm pretty sure tagged unions need coercion at some point to
		// convert from less general to more general tagged union type
		ty := ctx.simplify(e.ID())
		tu := assert.Cast[*types.TaggedUnion](ctx.resolveTypeApplications(ty), "BUG")
		ilTy, _ := ctx.taggedUnionToILType(tu)
		tagValue := tagValue(tu, e.Tag)

		payload := gen(ctx, e.Body)
		payloadTy := ctx.toILType(ctx.simplify(e.Body.ID()))

		return genRecordLiteral(ctx, ilTy, false, []recordAssignment{
			{name: "tag", typ: ctx.ptrType, value: qbeil.IntLiteral{Value: int64(tagValue)}},
			{name: "payload", typ: payloadTy, value: payload},
		})
	case *parse.CaseExpr:
		return genCaseExpr(ctx, e)
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
func genFunc(
	ctx *ctx,
	class string,
	expr *parse.FuncDef,
	closure bool, // whether this function is a closure. Adds a capture block function argument
	captureBlock *qbeil.StructType, // only used for closures. May be nil if there's no capture data
	capturesType map[string]types.Type, // only used for closures
) (val qbeil.Value) {
	// TODO: I should just set funcInProcess here directly
	// assert.Neq(ctx.funcInProcess, "", "BUG codegen: genFunc called without funcInProcess")
	defer func() {
		if e := recover(); e != nil {
			if class != "" {
				class = class + "#"
			}
			panic(fmt.Sprintf("in function %s%s: %v", class, expr.Name, e))
		}
	}()

	ctx.vars.NewScope()

	friendlyName := fmt.Sprintf("%s.%s", class, expr.Name)
	var realArgTys []types.Type
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
		ctx.vars.Insert(expr.Args[i], varInfo{
			uname:   expr.Args[i],
			val:     val,
			typ:     argTyp,
			lastUse: opt.None[parse.ID](),
		})
	}
	// closures take an extra void* for captures
	if closure {
		argTyps = append(argTyps, qbeil.NewTypedVar(ctx.ptrType, qbeil.Var{
			Global: false,
			Name:   "_captures",
		}))
	}

	// TODO: don't export all symbols
	linkage := qbeil.Linkage{Type: qbeil.Export}
	retTyp := ctx.toABIType(retTy)
	if expr.Name == "main" && class == "" {
		linkage.Type = qbeil.Export
		retTyp = qbeil.Word
	}

	ctx.il.Func(linkage, retTyp, thisFunc.IL(), argTyps)

	// re-expose captures as normal variables by emulating let bindings
	// TODO: can we merge into gen(LetExpr) code?
	if closure && captureBlock != nil {
		ctx.vars.NewScope()
		defer ctx.vars.PopScope()

		capturesArg := qbeil.Var{Global: false, Name: "_captures"}
		type layoutInfo struct {
			name   string
			offset int
		}
		fields := make([]layoutInfo, 0, len(captureBlock.Layouts))
		for field, layout := range captureBlock.Layouts {
			fields = append(fields, layoutInfo{
				name:   field,
				offset: layout.OffsetBits,
			})
		}
		slices.SortFunc(fields, func(a layoutInfo, b layoutInfo) int {
			if d := cmp.Compare(a.offset, b.offset); d != 0 {
				return d
			}
			// probably not possible but just in case
			return cmp.Compare(a.name, b.name)
		})

		for _, field := range fields {
			val := genRecordAccess(ctx, capturesArg, *captureBlock, field.name)
			ctx.vars.Insert(field.name, varInfo{
				uname:   val.Name,
				val:     val,
				typ:     capturesType[field.name],
				lastUse: opt.None[parse.ID](),
			})
		}
	}

	for _, stmt := range expr.Body[:len(expr.Body)-1] {
		gen(ctx, stmt)
	}

	ret := gen(ctx, expr.Body[len(expr.Body)-1])

	ctx.il.Ret(ret)
	ctx.il.EndFunc()
	// NOTE: arguments are borrowed by default, and caller is responsible for
	// increasing owned references.
	ctx.vars.PopScope()
	return thisFunc
}

func genCall(ctx *ctx, expr *parse.Form) qbeil.Value {
	assert.GreaterThan(len(expr.Children), 0, "empty form")

	switch callee := expr.Children[0].(type) {
	case *parse.New:
		ctorTy := ctx.simplify(callee.ID())
		ctorFn := assert.Cast[*types.Func](ctorTy, "BUG: constructor not typed as a function?")

		var classTy *types.Class
		switch ty := ctorFn.Ret.(type) {
		case *types.Class:
			classTy = ty
			// classIL = ctx.ensureClassDeclared(ty)
		case *types.Application:
			classTy = assert.Cast[*types.Class](ctx.typeApplicationToType(ty),
				"codegen: new called with a non-class %s", ty)
			// classIL = ctx.ensureClassDeclared(classTy)
		default:
			panic("BUG: constructor returns a non Class or Application type? " + ctorFn.Ret.String())
		}

		val := ctx.il.TempVar(false)
		ctx.il.Call(
			&val,
			ctx.ptrType,
			qbeil.Var{Global: true, Name: mangledNew(classTy.Module, classTy.Name)},
			[]qbeil.ABITypedValue{},
		)

		return val

	case *parse.MethodAccess:
		ty := ctx.resolveTypeApplications(ctx.simplify(callee.Obj.ID()))
		var mod can.ModuleName
		var class string
		switch t := ty.(type) {
		case *types.Class:
			// HACK: I really shouldn't be walking the type hierarchy in codegen...
			methOwner, ok := ctx.findMethodSource(t, callee.Method)
			assert.True(ok, "BUG codegen: could not find owner of method", callee.Method,
				"in:", expr.Pretty())
			mod = methOwner.Module
			class = methOwner.Name
		case *types.Record:
			panic("unimplemented: method call on record types")
		default:
			panic(fmt.Sprintf("unexpected LHS type of method accessor %T in: %s", ty, callee))
		}
		assert.Neq(class, "", "unnamed class not yet supported")
		return genCallWithFuncName(ctx, mod, class, callee.Method, expr)

	case *parse.RecordAccess:
		// currently supports:
		// 1. (Module.Type.staticMethod x y z)
		// 2. (Module.function x y z)
		// 3. (LocalClass.staticMethod x y z)
		// 4. (recordVar.field x y z)
		modulePath := ctx.module
		var class string
		switch lhs := callee.Record.(type) {
		case *parse.Symbol:
			if modName, ok := ctx.imports[lhs.Name]; ok {
				// Module.function
				modulePath = modName
			} else if _, ok := ctx.allModules[ctx.module].Types[lhs.Name]; ok {
				// LocalClass.staticMethod
				// TODO: check if this is actually a class
				class = lhs.Name
			} else {
				// recordVar.field
				// TODO: this could be an object?
				lhsTy := assert.Cast[qbeil.StructType](ctx.toILType(ctx.simplify(lhs.ID())),
					"BUG codegen: expected LHS of callee", callee.Pretty(), "to be a record type")
				lhsRcd := gen(ctx, lhs)
				closurePtr := genGetStructPtrField(ctx, lhsRcd, lhsTy, callee.Field)

				return genCallClosure(ctx, closurePtr, expr)
			}
		case *parse.RecordAccess:
			// Module.Type.staticMethod
			llhs := assert.Cast[*parse.Symbol](lhs.Record,
				"Currently does not support calling record fields. How did you get here?")
			modulePath = can.ModuleName(llhs.Name)
			modName := assert.Get(ctx.imports, llhs.Name,
				"Only module name is allowed here, did you try to call a record field?")
			class = lhs.Field
			mod := assert.Get(ctx.allModules, modName, "BUG imported module is missing")
			assert.Get(mod.Types, class,
				"Only static method call on imported type allowed here, did you try to call a record field?")
		default:
			panic(fmt.Sprintf("unexpected lhs of record accessor %T in: %s", callee.Record, callee))
		}

		return genCallWithFuncName(ctx, modulePath, class, callee.Field, expr)

	case *parse.Symbol:
		if closure, ok := ctx.vars.Get(callee.Name).Unwrap(); ok {
			// TODO: is it always a closure?
			return genCallClosure(ctx, closure.val, expr)
		}
		return genCallWithFuncName(ctx, ctx.module, "", callee.Name, expr)

	case *parse.Fn:
		closure := genClosure(ctx, callee)
		return genCallClosure(ctx, closure, expr)

	default:
		panic("unimpl: genCall for callee of the form " + expr.Pretty())
	}
}

func genCallWithFuncName(ctx *ctx, module can.ModuleName, class string, fnName string, expr *parse.Form) qbeil.Value {
	mangled := fnName
	switch fnName {
	case "strFromCStr", "strToCStr", "i64ToI32", "i64ToI8", "i64ToStr", "emptyList": // don't mangle
	case "typeOf":
		// derived from G_OBJECT_TYPE:
		// gtype(instance) = (instance as GTypeInstance*)->g_class->g_type
		assert.Eq(len(expr.Children), 2, "genqbe: wrong arg count in typeOf")
		ty := ctx.resolveTypeApplications(ctx.simplify(expr.Children[1].ID()))
		assert.Cast[*types.Class](ty, "genqbe: typeOf called on non class type?")
		val := gen(ctx, expr.Children[1])
		gClass := genGetStructPtrField(ctx, val, ctx.gObject(), "g_class")
		gType := genGetStructPtrField(ctx, gClass, ctx.gTypeClass(), "g_type")
		return gType
	default:
		if !mapHas(ctx.externs, fnName) /* FIXME: might get shadowed */ {
			mangled = mangleName(mangleOpts{module: module, class: class, name: fnName})
		}
	}

	fnFriendlyName := fnName
	if class != "" {
		fnFriendlyName = fmt.Sprintf("%s.%s", class, fnName)
	}
	callee := expr.Children[0]

	switch fnName {
	case "+", "-", "*", "/":
		assert.Eq(len(expr.Children), 3, "wrong function arg count")

		instr := getArithmeticInstruction(fnName)
		left := gen(ctx, expr.Children[1])
		right := gen(ctx, expr.Children[2])
		target := ctx.il.TempVar(false)
		ctx.il.Arithmetic(target.IL(), qbeil.Long, instr, left, right)

		return target
	case "exit":
		assert.Eq(len(expr.Children), 2, "wrong function arg count")

		exitCode := gen(ctx, expr.Children[1])
		ctx.il.Call(nil, qbeil.Long, qbeil.Var{Global: true, Name: "exit"}, []qbeil.ABITypedValue{
			{Type: ctx.intType, Value: exitCode},
		})
		return qbeil.IntLiteral{Value: 0} // TODO: there's probably a better way
	case "assert":
		assert.Eq(len(expr.Children), 2, `wrong arg count for "assert"`)

		msgStr := "assertion failed: " + expr.Pretty()
		msgGlobal := ctx.il.TempNamedVar(true, "assert_msg")
		ctx.statics[msgGlobal] = fmt.Sprintf(`{b "%s", b 0}`, msgStr)
		strTy := assert.Cast[qbeil.StructType](ctx.userTypes["Str"])

		printFunc := qbeil.Var{Global: true, Name: "print"}
		failLabel := ctx.il.TempLabel("assert_fail_")
		okLabel := ctx.il.TempLabel("assert_pass_")

		assertionResult := gen(ctx, expr.Children[1])

		ctx.il.Jnz(assertionResult, okLabel, failLabel)

		ctx.il.Label(failLabel.Name)
		msg := genRecordLiteral(ctx, strTy, false, []recordAssignment{
			{name: "data", typ: ctx.ptrType, value: msgGlobal},
			{name: "size", typ: ctx.ptrType, value: qbeil.IntLiteral{Value: int64(len(msgStr))}},
		})
		ctx.il.Call(nil, nil, printFunc, []qbeil.ABITypedValue{
			{Type: strTy, Value: msg},
		})
		ctx.il.Call(nil, nil, qbeil.Var{Global: true, Name: "exit"}, []qbeil.ABITypedValue{
			{Type: ctx.intType, Value: qbeil.IntLiteral{Value: 1}},
		})

		ctx.il.Label(okLabel.Name)
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

		switch ilTy.(type) {
		case qbeil.BaseType:
			ctx.il.Command("store"+ilTy.IL(), qbeil.IntLiteral{Value: 0}, dataPtr)
		case qbeil.ExtraType:
			panic("TODO stackAlloc ExtType")
		case qbeil.StructType, qbeil.UnionType:
			// TODO: zero out
		default:
			panic(fmt.Sprintf("unexpected qbeil.Type: %#v", ilTy))
		}

		return dataPtr

	case "ptrCast":
		assert.Eq(len(expr.Children), 2, `wrong arg count for "ptrCast"`)
		return gen(ctx, expr.Children[1])

	case "ref":
		assert.Eq(len(expr.Children), 2, `wrong arg count for "ref"`)
		ty := ctx.simplify(expr.Children[1].ID())
		ilTy := ctx.toILType(ty)
		val := gen(ctx, expr.Children[1])

		switch ilTy.(type) {
		case qbeil.BaseType:
			bits, _ := ctx.sizeOf(ilTy)
			ptr := ctx.il.TempNamedVar(false, "ref")
			ctx.il.Arithmetic(ptr.IL(), ctx.ptrType, "alloc4", qbeil.IntLiteral{Value: int64(bits / 8)})
			ctx.il.Command("store"+ilTy.IL(), val, ptr)
			return ptr
		case qbeil.ExtraType:
			panic("TODO ref on ExtraType")
		case qbeil.StructType, qbeil.UnionType:
			return val
		default:
			panic(fmt.Sprintf("unexpected qbeil.Type: %#v", ilTy))
		}

	case "deref":
		assert.Eq(len(expr.Children), 2, `wrong arg count for "deref"`)

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

		switch ilTy1 := ilTy.(type) {
		case qbeil.BaseType:
			derefed := ctx.il.TempNamedVar(false, "derefed")
			ctx.il.Arithmetic(derefed.IL(), ilTy1, "load"+ilTy1.IL(), ref)
			return derefed
		case qbeil.ExtraType:
			// loadsh, loadsb, etc. returns long or word, can't use those
			stackPtr := ctx.il.TempNamedVar(false, "derefedPtr")
			ctx.il.Arithmetic(stackPtr.IL(), ctx.ptrType, "alloc4", qbeil.IntLiteral{Value: int64(bits / 8)})
			ctx.il.Command("blit", ref, stackPtr, qbeil.IntLiteral{Value: int64(bits / 8)})
			return stackPtr
		case qbeil.AggregateType: // struct or union
			stackPtr := ctx.il.TempNamedVar(false, "derefedPtr")
			ctx.il.Arithmetic(stackPtr.IL(), ctx.ptrType, "alloc4", qbeil.IntLiteral{Value: int64(bits / 8)})
			ctx.il.Command("blit", ref, stackPtr, qbeil.IntLiteral{Value: int64(bits / 8)})
			return stackPtr
		default:
			panic(fmt.Sprintf("unknown IL type %v", ilTy))
		}

	case "retain":
		assert.Eq(len(expr.Children), 2, `wrong arg count for "retain"`)

		// TODO: actually increase ref count for other types
		ty := ctx.simplify(expr.Children[1].ID())
		val := gen(ctx, expr.Children[1])

		// TODO: error when called on value types?
		genRef(ctx, val, ty, "")

		return val

	case "own":
		assert.Eq(len(expr.Children), 2, `wrong arg count for "own"`)

		ty := ctx.simplify(expr.Children[1].ID())
		ownTy, ok := ty.(*types.ExplicitOwnership)
		if !ok {
			panic(fmt.Sprintf("during codegen: `own` expects an Unowned type, got: %v", ty))
		}

		assert.Eq(ownTy.Kind, parse.Unowned,
			"during codegen: `own` expects first argument to be an Unowned type, got: %s", ownTy.String())

		// TODO: actually increase ref count
		return gen(ctx, expr.Children[1])

	case "toCClosure", "ptrToI64":
		assert.Eq(len(expr.Children), 2, "BUG", fnName, "wrong arg count at code gen")
		return gen(ctx, expr.Children[1])

	case "=":
		assert.Eq(len(expr.Children), 3, "BUG: wrong arg count at code gen for equality operator")
		lhs := gen(ctx, expr.Children[1])
		rhs := gen(ctx, expr.Children[2])
		res := ctx.il.TempNamedVar(false, "equalityResult")
		retTy := ctx.toILType(ctx.simplify(expr.ID()))
		ctx.il.Arithmetic(res.IL(), retTy, "ceq"+retTy.IL(), lhs, rhs)
		return res

	case "at":
		assert.Eq(len(expr.Children), 3, "BUG", fnName, "wrong arg count at code gen")
		listTy := assert.Cast[*types.Slice](ctx.simplify(expr.Children[1].ID()))
		contentTy := listTy.Type
		listStruct := assert.Get(ctx.userTypes, "List",
			"BUG codegen: List struct type not defined?")
		bits, _ := ctx.sizeOf(ctx.toILType(contentTy))
		size := bitsToBytesRoundedUp(bits)
		sizeVal := qbeil.IntLiteral{Value: int64(size)}
		funcVar := qbeil.Var{Global: true, Name: "listGet"}

		resultPtr := ctx.il.TempNamedVar(false, "at_result_ptr")

		list := gen(ctx, expr.Children[1])
		idx := gen(ctx, expr.Children[2])
		ctx.il.Arithmetic(resultPtr.IL(), ctx.ptrType, "alloc4", sizeVal)

		ctx.il.Call(nil, nil, funcVar, []qbeil.ABITypedValue{
			{Type: listStruct, Value: list},
			{Type: qbeil.Long, Value: sizeVal},
			{Type: qbeil.Long, Value: idx},
			{Type: ctx.ptrType, Value: resultPtr},
		})

		// deref if base type, (maybe can merge with deref code)
		baseTy, ok := ctx.toILType(contentTy).(qbeil.BaseType)
		if !ok {
			return resultPtr
		}

		resultVar := ctx.il.TempNamedVar(false, "at_result")

		ctx.il.Arithmetic(resultVar.IL(), baseTy, "load"+baseTy.IL(), resultPtr)

		return resultVar

	case "forEach":
		assert.Eq(len(expr.Children), 3, "BUG", fnName, "wrong arg count at code gen")
		listTy := assert.Cast[*types.Slice](ctx.simplify(expr.Children[1].ID()))
		contentTy := listTy.Type
		listStruct := assert.Get(ctx.userTypes, "List",
			"BUG codegen: List struct type not defined?")
		closureStruct := assert.Get(ctx.userTypes, "ClosureComponents",
			"BUG codegen: ClosureComponents struct type not defined?")
		bits, _ := ctx.sizeOf(ctx.toILType(contentTy))
		size := bitsToBytesRoundedUp(bits)
		sizeVal := qbeil.IntLiteral{Value: int64(size)}
		funcVar := qbeil.Var{Global: true, Name: "listForEach"}

		list := gen(ctx, expr.Children[1])
		closure := gen(ctx, expr.Children[2])

		ctx.il.Call(nil, nil, funcVar, []qbeil.ABITypedValue{
			{Type: listStruct, Value: list},
			{Type: qbeil.Long, Value: sizeVal},
			{Type: closureStruct, Value: closure},
		})

		// should be empty record
		return qbeil.IntLiteral{Value: 0}

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
		// skip callee expression
		argExprs := expr.Children[1:]
		// skip type of self, if applicable
		nonSelfTys := realArgTys[len(args):]
		for arg, typ := range fun.ZipSlicesStrict(argExprs, nonSelfTys) {
			args = append(args, qbeil.ABITypedValue{Type: ctx.toABIType(typ), Value: gen(ctx, arg)})
		}
		funcVar := qbeil.Var{Global: true, Name: mangled}

		ctx.il.Call(&target, ctx.toABIType(funcSig.Ret), funcVar, args)

		return target
	}
}

func getArithmeticInstruction(op string) string {
	switch op {
	case "+":
		return "add"
	case "-":
		return "sub"
	case "*":
		return "mul"
	case "/":
		return "div"
	default:
		panic("unknown operator: " + op)
	}
}

func genClosure(ctx *ctx, e *parse.Fn) qbeil.Value {
	assert.Neq(ctx.funcInProcess, "",
		"Empty ctx.funcInProcess. Top-level fn (i.e. outside of a def) is not allowed")
	name := ctx.newTempName(ctx.funcInProcess + ".fn")
	className := ""
	if ctx.classInProcess != nil {
		className = ctx.classInProcess.Name
	}

	// assign capture block
	captures := assert.Get(ctx.allModules[ctx.module].Captures, e.ID(),
		"BUG codegen: a closure has no capture group: ", e.String())
	blockFields := ordered.NewMap[string, types.Type]()
	for _, capture := range captures {
		blockFields.Insert(capture.Name, ctx.simplify(capture.ID))
	}
	fields := fun.Map(captures, func(c simplesub.Capture) recordAssignment {
		val, ok := ctx.vars.Get(c.Name).Unwrap()
		assert.True(ok,
			"could not find local variable", c.Name, "while building capture block")
		return recordAssignment{
			name:  c.Name,
			typ:   ctx.toILType(ctx.simplify(c.ID)),
			value: val.val,
		}
	})

	captureTypes := make(map[string]types.Type)
	for _, c := range captures {
		captureTypes[c.Name] = ctx.simplify(c.ID)
	}
	var captureBlock qbeil.Value
	var captureBlockIlTy *qbeil.StructType
	if blockFields.Len() == 0 {
		// currently empty records have IL type of ptrType
		captureBlock = qbeil.IntLiteral{Value: 0}
	} else {
		captureBlockTy := types.Record{
			Fields: blockFields,
		}
		c := ctx.recordToILType(&captureBlockTy)
		captureBlockIlTy = &c

		ctx.il.Comment("capture data: (Ref %s) => pointer to %s", captureBlockTy.String(), captureBlockIlTy.IL())
		captureBlock = genRecordLiteral(ctx, c, true, fields)
	}

	funcPtr := qbeil.Var{
		Global: true,
		Name: mangleName(mangleOpts{
			module: ctx.module,
			class:  className,
			name:   name,
		}),
	}

	// assemble ClosureComponents struct
	closureTy := assert.Get(ctx.userTypes, "ClosureComponents",
		"BUG codegen: ClosureComponents not declared?")
	closureSTy := assert.Cast[qbeil.StructType](closureTy,
		"BUG codegen: ClosureComponents is not a StructType?")
	closureComponents := genRecordLiteral(ctx, closureSTy, false, []recordAssignment{
		{name: "func", typ: ctx.ptrType, value: funcPtr},
		{name: "data", typ: ctx.ptrType, value: captureBlock},
		// TODO
		{name: "cleanup", typ: ctx.ptrType, value: qbeil.IntLiteral{Value: 0}},
	})

	ctx.unprocessedClosures = append(ctx.unprocessedClosures, closure{
		name:         name,
		expr:         e,
		captureData:  captureBlockIlTy,
		captureTypes: captureTypes,
	})
	return closureComponents
}

func genCallClosure(ctx *ctx, closureComponents qbeil.Value, expr *parse.Form) qbeil.Value {
	// TODO: merge with default case of genCallWithFuncName
	ilTy := assert.Get(ctx.userTypes, "ClosureComponents",
		"BUG codegen ClosureComponents not declared")
	structTy := ilTy.(qbeil.StructType)
	funcPtr := genRecordAccess(ctx, closureComponents, structTy, "func")

	args := make([]qbeil.ABITypedValue, 0, len(expr.Children))
	for _, arg := range expr.Children[1:] {
		argVal := gen(ctx, arg)
		argTy := ctx.toABIType(ctx.simplify(arg.ID()))
		args = append(args, qbeil.ABITypedValue{
			Type:  argTy,
			Value: argVal,
		})
	}

	// pass captures as the last argument
	captures := genRecordAccess(ctx, closureComponents, structTy, "data")
	args = append(args, qbeil.ABITypedValue{
		Type:  ctx.ptrType,
		Value: captures,
	})

	ret := ctx.il.TempVar(false)
	retType := ctx.toABIType(ctx.simplify(expr.ID()))
	ctx.il.Call(&ret, retType, funcPtr, args)
	return ret
}

func genLet(ctx *ctx, expr *parse.LetExpr) qbeil.Value {
	ctx.vars.NewScope()

	for _, ass := range expr.Assignments {
		ctx.il.Comment("let assignment of %s = %s",
			ass.Var, ass.Value.Pretty())

		rhsVal := gen(ctx, ass.Value)

		// TODO: no reason to copy stuff that does not need gc
		// I should refactor varInfo to take optional garbage
		// collection info
		ctx.il.Comment("reassign to var name for garbage collection later")
		varPtr := qbeil.Var{Global: false, Name: ctx.newTempName(ass.Var)}
		varTy := ctx.simplify(ass.Value.ID())
		varIlTy := ctx.toILType(varTy)
		varSizeBits, _ := ctx.sizeOf(varIlTy)
		ctx.il.Arithmetic(varPtr.IL(), ctx.ptrType, "alloc4", qbeil.IntLiteral{
			Value: int64(bitsToBytesRoundedUp(varSizeBits)),
		})
		genCopyToPtr(ctx, varPtr, varIlTy, rhsVal)

		genRef(ctx, rhsVal, varTy, "let_"+ass.Var)
		ctx.vars.Insert(ass.Var, varInfo{
			uname:   varPtr.Name,
			val:     genReturnableValueFromPtr(ctx, varPtr, varIlTy),
			typ:     ctx.simplify(ass.Value.ID()),
			lastUse: opt.None[parse.ID](),
		})
	}

	ctx.il.Comment("let body")
	ret := gen(ctx, expr.Body)
	genUnrefAndPopScope(ctx, "let")

	return ret
}

func genIf(ctx *ctx, expr *parse.IfExpr) qbeil.Value {
	thenLabel := ctx.il.TempLabel("then_")
	elseLabel := ctx.il.TempLabel("else_")
	endLabel := ctx.il.TempLabel("endif_")

	retType := ctx.toILType(ctx.simplify(expr.ID()))
	retSizeBits, _ := ctx.sizeOf(retType)
	retPtr := ctx.il.TempNamedVar(false, "if_expr_val")

	ctx.il.Arithmetic(retPtr.IL(), ctx.ptrType, "alloc4", qbeil.IntLiteral{
		Value: int64(retSizeBits / 8),
	})

	cond := gen(ctx, expr.Condition)
	ctx.il.Jnz(cond, thenLabel, elseLabel)

	ctx.il.Label(thenLabel.Name)
	thenVal := gen(ctx, expr.Consequence)
	genCopyToPtr(ctx, retPtr, retType, thenVal)
	ctx.il.Jump(endLabel)

	ctx.il.Label(elseLabel.Name)
	elseVal := gen(ctx, expr.Alternative)
	genCopyToPtr(ctx, retPtr, retType, elseVal)
	ctx.il.Jump(endLabel)

	ctx.il.Label(endLabel.Name)
	return genReturnableValueFromPtr(ctx, retPtr, retType)
}

func genCaseExpr(ctx *ctx, e *parse.CaseExpr) qbeil.Value {
	endLabel := ctx.il.TempLabel("endcase_")

	matchTy := ctx.simplify(e.Match.ID())
	matchTu := assert.Cast[*types.TaggedUnion](ctx.resolveTypeApplications(matchTy),
		"BUG matched expression in case expression is not a tagged union?")
	matchIlTy, _ := ctx.taggedUnionToILType(matchTu)

	retType := ctx.toILType(ctx.simplify(e.ID()))
	retSizeBits, _ := ctx.sizeOf(retType)
	retPtr := ctx.il.TempNamedVar(false, "case_expr_val")

	ptrSizeBits, _ := ctx.sizeOf(ctx.ptrType)
	ptrSize := int64(bitsToBytesRoundedUp(ptrSizeBits))

	ctx.il.Arithmetic(retPtr.IL(), ctx.ptrType, "alloc4", qbeil.IntLiteral{
		Value: int64(bitsToBytesRoundedUp(retSizeBits)),
	})

	match := gen(ctx, e.Match)
	tag := genGetStructPtrField(ctx, match, matchIlTy, "tag")

	for _, branch := range e.Branches {
		pattTag := tagValue(matchTu, branch.Pattern.Tag)

		checkRes := ctx.il.TempNamedVar(false, "tagComparisonResult")
		checkResTy := ctx.ptrType
		ctx.il.Comment("match tag '%s", branch.Pattern.Tag)
		ctx.il.Arithmetic(checkRes.IL(), checkResTy, "ceq"+checkResTy.IL(),
			tag, qbeil.IntLiteral{Value: pattTag})

		matchOkLabel := ctx.il.TempLabel("case_matched_" + branch.Pattern.Tag)
		matchFailLabel := ctx.il.TempLabel("case_failed_" + branch.Pattern.Tag)
		ctx.il.Jnz(checkRes, matchOkLabel, matchFailLabel)

		ctx.il.Label(matchOkLabel.Name)
		ctx.vars.NewScope()

		ctx.il.Comment("assign tag payload")
		maybePayloadTy, ok := matchTu.Variants.Get(branch.Pattern.Tag)
		assert.True(ok, "BUG codegen: type is missing tag", branch.Pattern.Tag)
		payloadTy, ok := maybePayloadTy.Unwrap()
		assert.True(ok, "tag without payload is not yet supported (how did you get here?)")
		payloadIlTy := ctx.toILType(payloadTy)

		payloadPtr := ctx.il.TempNamedVar(false, "payloadPtr")
		ctx.il.Arithmetic(payloadPtr.IL(), ctx.ptrType, "add",
			match, qbeil.IntLiteral{Value: ptrSize})
		payload := genReturnableValueFromPtr(ctx, payloadPtr, payloadIlTy)
		genRef(ctx, payload, payloadTy, "case")
		ctx.vars.Insert(branch.Pattern.Pattern.Name, varInfo{
			uname:   payloadPtr.Name,
			val:     payload,
			typ:     payloadTy,
			lastUse: opt.None[parse.ID](),
		})

		val := gen(ctx, branch.Body)
		genCopyToPtr(ctx, retPtr, retType, val)
		genUnrefAndPopScope(ctx, "case")
		ctx.il.Jump(endLabel)

		ctx.il.Label(matchFailLabel.Name)
	}

	ctx.il.Label(endLabel.Name)
	return genReturnableValueFromPtr(ctx, retPtr, retType)
}

func genClassDef(ctx *ctx, e *parse.ObjectTypeDef) {
	// TODO: support generics
	ct := ctx.simplify(e.ID())
	classTy, ok := ct.(*types.Class)
	if !ok {
		panic(fmt.Sprintf("compiler bug: class definition yields non-class type %#v", ct))
	}

	class := ctx.classToILType(classTy)
	ctx.declareType(e.Name, class)

	genMethodDefs(ctx, e, classTy)

	if e.Extern {
		return
	}

	classType := ctx.ensureObjectTypeDeclared(classTy)
	parents := fun.Map(classTy.Supers, func(sup *types.Application) *types.Class {
		p := assert.Cast[*types.Class](ctx.typeApplicationToType(sup),
			"codegen: a non-class/interface type %s.%s is used as a super of %s",
			sup.Module, sup.Name, classTy.Name)
		return p
	})

	private := qbeil.StructType{
		Name: e.Name + "Private",
		// TODO: handle privates
		Fields: []qbeil.RepeatType{},
	}
	ctx.declareType(e.Name+"Private", private)

	genObjectTypeBoilerplate(ctx, objectTypeBoilerplateOpt{
		class:             classTy,
		maybeInstanceType: class,
		classType:         classType,
		maybePrivate:      private,
		iface:             false,
		extern:            e.Extern,
		parents:           parents,
	})
}

func genMethodDefs(ctx *ctx, e *parse.ObjectTypeDef, classTy *types.Class) {
	ctx.classInProcess = classTy // TODO: refactor this out somehow
	defer func() { ctx.classInProcess = nil }()

	for _, field := range e.Fields {
		method, ok := field.(parse.ClassMethod)
		if !ok {
			continue
		}

		genFunc(ctx, classTy.Name, method.Func, false, nil, nil)
	}
}

type objectTypeBoilerplateOpt struct {
	class             *types.Class
	maybeInstanceType qbeil.AggregateType
	classType         qbeil.AggregateType
	maybePrivate      qbeil.AggregateType
	iface             bool // TODO: remove, use class.Kind instead
	extern            bool
	parents           []*types.Class
}

// generates functions that take care of initialization
// e.g. function to retrieve the GType of the class
// NOTE: only generate for types owned by ctx.module
func genObjectTypeBoilerplate(ctx *ctx, opt objectTypeBoilerplateOpt) {
	if !opt.iface {
		genClassInitializeIfacesFuncs(ctx, opt.class, opt.parents)
	}

	genObjectTypeGetTypeFunc(ctx, opt)
	genObjectTypeConstructor(ctx, opt)
	if !opt.iface && !opt.extern {
		// TODO: skip new() on abstract classes
		genClassNewFunc(ctx, opt)
	}
}

func genObjectTypeGetTypeFunc(ctx *ctx, opt objectTypeBoilerplateOpt) {
	classBits := 0
	if opt.maybeInstanceType != nil {
		classBits, _ = ctx.sizeOf(opt.maybeInstanceType)
	}
	classTypeBits, _ := ctx.sizeOf(opt.classType)
	privateBits := 0
	if opt.maybePrivate != nil {
		privateBits, _ = ctx.sizeOf(opt.maybePrivate)
	}

	typeNameVar := ctx.il.StrData(qbeil.DataDef{
		Linkage: qbeil.Linkage{},
		VarName: mangleName(mangleOpts{
			module: ctx.module,
			class:  opt.class.Name,
			name:   "class_name",
		}),
	}, opt.class.Name)

	// TODO: name collision?
	typeIdVarOnce := ctx.il.Data(
		qbeil.DataDef{
			Linkage: qbeil.Linkage{},
			VarName: mangleName(mangleOpts{
				module: ctx.module,
				class:  opt.class.Name,
				name:   "_type_id__once",
			}),
			Align: 0,
		},
		ctx.ptrType, // TODO: is this correct? gsize == guintptr??
		qbeil.IntLiteral{Value: 0},
	)

	typeIdTempVar := ctx.il.TempVar(false)

	typeInfoConst := genConstDefineTypeInfo(ctx, typeInfoOpt{
		className:    opt.class.Name,
		classSize:    uint16(classTypeBits / 8),
		instanceSize: uint16(classBits / 8),
	})

	// get_type function

	ctx.il.Func(
		qbeil.Linkage{Type: qbeil.Export},
		ctx.ptrType, /* GType */
		"$"+mangledClassTypeGetter(ctx.module, opt.class.Name),
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
	{
		gtype := G_TYPE_OBJECT
		if opt.iface {
			gtype = G_TYPE_INTERFACE
		}
		ctx.il.Call(
			&typeIdTempVar,
			// return type is GType
			ctx.ptrType,
			qbeil.Var{Global: true, Name: "g_type_register_static"},
			[]qbeil.ABITypedValue{
				{Type: ctx.ptrType /* GType */, Value: qbeil.IntLiteral{Value: int64(gtype)}},
				{Type: ctx.ptrType, Value: typeNameVar},
				{Type: ctx.ptrType, Value: typeInfoConst},
				// TODO: this is an enum GTypeFlags, idk what type it should be
				{Type: ctx.intType, Value: qbeil.IntLiteral{Value: 0}},
			},
		)

		// register interfaces
		// TODO: interfaces extending other ifaces
		if opt.iface {
			addPrereq := qbeil.Var{Name: "g_type_interface_add_prerequisite", Global: true}
			ctx.il.Call(nil, nil, addPrereq, []qbeil.ABITypedValue{
				{Type: ctx.ptrType, Value: typeIdTempVar},
				{Type: ctx.ptrType, Value: qbeil.IntLiteral{Value: int64(G_TYPE_OBJECT)}},
			})
		} else {
			for _, parent := range opt.parents {
				if parent.Kind == parse.Iface {
					genObjectTypeIfaceInitFunc(ctx, typeIdTempVar, parent)
				}
			}
		}

		// TODO: classes implementing interface?

		if privateBits > 0 {
			privateOffset := ctx.il.Data(
				qbeil.DataDef{
					Linkage: qbeil.Linkage{},
					VarName: opt.class.Name + "_private_offset",
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
	}

	ctx.il.InsertLabel(elseLabel)
	gtypeVar := ctx.il.TempNamedVar(false, "gtype_val")
	ctx.il.Arithmetic(gtypeVar.IL(), ctx.ptrType, "load"+ctx.ptrType.IL(), typeIdVarOnce)
	ctx.il.Ret(gtypeVar)

	ctx.il.EndFunc()
}

func genClassInitializeIfacesFuncs(ctx *ctx, class *types.Class, parents []*types.Class) {
	assert.Eq(class.Kind, parse.Class, "codegen", callerName(), ": class must not be an interface")
	className := typeName{class.Module, class.Name}

	for _, parent := range parents {
		if parent.Kind != parse.Iface {
			continue
		}

		initName := mangledClassIfaceInit(className, typeName{parent.Module, parent.Name})

		objParentIfaceName := fmt.Sprintf("%s%sIface", class.Name, parent.Name)
		objParentIface := ctx.il.Data(qbeil.DataDef{
			Linkage: qbeil.Linkage{},
			VarName: objParentIfaceName,
			Align:   0,
		}, ctx.ptrType, qbeil.IntLiteral{Value: 0})

		{
			self := qbeil.Var{Name: "self"}
			klass_data := qbeil.Var{Name: "klass_data"}
			ctx.il.Func(qbeil.Linkage{Type: qbeil.Export}, nil, initName, []qbeil.TypedVar{
				{Type: ctx.ptrType, Name: self},
				{Type: ctx.ptrType, Name: klass_data},
			})
			interface_peek_parent := qbeil.Var{Global: true, Name: "g_type_interface_peek_parent"}
			// TODO: what is this for?
			ctx.il.Call(&objParentIface, ctx.ptrType, interface_peek_parent, []qbeil.ABITypedValue{
				{Type: ctx.ptrType, Value: self},
			})

			// assign methods to vtable
			for _, parent := range parents {
				// TODO: all methods of ancestors of parent
				ifaceType := ctx.ensureObjectTypeDeclared(parent)
				for meth := range ifaceType.Layouts {
					_, ok := class.Methods.Get(meth)
					if !ok {
						continue // TODO: is this an error?
					}

					// TODO: do I need to handle different named mangling schemes? i.e. vala's
					methPtr := qbeil.Var{
						Global: true,
						Name: mangleName(mangleOpts{
							module: class.Module,
							class:  class.Name,
							name:   meth,
						})}

					genSetStructPtrField(ctx, self, ifaceType, meth, methPtr)
				}
			}

			ctx.il.EndFunc()
		}
	}
}

func genObjectTypeIfaceInitFunc(ctx *ctx, objectTypeId qbeil.Var, parent *types.Class) {
	assert.Eq(parent.Kind, parse.Iface, "codegen", callerName(), ": parent is not an interface?")

	// const static meower_info = GInterfaceInfo{...}
	ifaceInfoData := genInterfaceInfo(ctx, gInterfaceInfo{
		module:         parent.Module,
		name:           parent.Name,
		maybeInit:      qbeil.Var{},
		maybeFinalize:  qbeil.Var{},
		maybeIfaceData: qbeil.Var{},
	})

	typeInst := ctx.il.TempVar(false)
	typeGetter := qbeil.Var{
		Global: false,
		Name:   mangledClassTypeGetter(parent.Module, parent.Name),
	}
	ctx.il.Call(&typeInst, ctx.ptrType, typeGetter, []qbeil.ABITypedValue{})

	// g_type_add_interface_static (cat_type_id, TYPE_MEOWER, &meower_info);
	addIface := qbeil.Var{Name: "g_type_add_interface_static", Global: true}
	ctx.il.Call(nil, nil, addIface, []qbeil.ABITypedValue{
		{Type: ctx.ptrType, Value: objectTypeId},
		{Type: ctx.ptrType, Value: typeInst},
		{Type: ctx.ptrType, Value: ifaceInfoData},
	})
}

type gInterfaceInfo struct {
	module         can.ModuleName
	name           string
	maybeInit      qbeil.DataItem
	maybeFinalize  qbeil.DataItem
	maybeIfaceData qbeil.DataItem
}

// generates GInterfaceInfo
func genInterfaceInfo(ctx *ctx, opt gInterfaceInfo) qbeil.Var {
	// TODO: also take class name (+interface name) for mangling
	if opt.maybeInit == nil {
		opt.maybeInit = qbeil.IntLiteral{Value: 0}
	}
	if opt.maybeFinalize == nil {
		opt.maybeFinalize = qbeil.IntLiteral{Value: 0}
	}
	if opt.maybeIfaceData == nil {
		opt.maybeIfaceData = qbeil.IntLiteral{Value: 0}
	}
	name := mangleName(mangleOpts{
		module: opt.module,
		class:  opt.name,
		name:   "iface_info",
	})
	ctx.il.CompositeData(qbeil.DataDef{
		Linkage: qbeil.Linkage{},
		VarName: name,
		Align:   0,
	}, qbeil.DataItems([]qbeil.TypedDataItem{
		{Type: ctx.ptrType, Value: opt.maybeInit},
		{Type: ctx.ptrType, Value: opt.maybeFinalize},
		{Type: ctx.ptrType, Value: opt.maybeIfaceData},
	}))

	return qbeil.Var{Global: true, Name: name}
}

func genObjectTypeConstructor(ctx *ctx, opt objectTypeBoilerplateOpt) {
	ctorName := mangleName(mangleOpts{
		module: ctx.module,
		class:  opt.class.Name,
		name:   "construct",
	})
	argObjType := qbeil.Var{Name: "object_type", Global: false}
	ctx.il.Func(qbeil.Linkage{Type: qbeil.Export}, ctx.ptrType, "$"+ctorName, []qbeil.TypedVar{
		{Type: /* GType */ ctx.ptrType, Name: argObjType},
	})
	obj := ctx.il.TempVar(false)

	// TODO: I'm not really sure how to tell if a class has a construct()
	// extern ones (almost?) certainly don't have, but I'll have to expose that...
	parentLacksConstruct := len(opt.parents) == 0 ||
		opt.parents[0].Module != ctx.module ||
		(opt.parents[0].Module == simplesub.StdModName &&
			opt.parents[0].Name == "Object")
	if parentLacksConstruct {
		parentCtorName := "g_object_new"
		ctx.il.Call(&obj, ctx.ptrType, qbeil.Var{Name: parentCtorName, Global: true}, []qbeil.ABITypedValue{
			{Type: ctx.ptrType, Value: argObjType},
			// TODO: install properties
			{Type: ctx.ptrType, Value: qbeil.IntLiteral{Value: 0}},
		})
	} else {
		parentCtorName := mangleName(mangleOpts{
			// FIXME: check not interface
			module: opt.parents[0].Module,
			class:  opt.parents[0].Name,
			name:   "construct",
		})
		ctx.il.Call(&obj, ctx.ptrType, qbeil.Var{Name: parentCtorName, Global: true}, []qbeil.ABITypedValue{
			{Type: ctx.ptrType, Value: argObjType},
		})
	}
	ctx.il.Ret(obj)
	ctx.il.EndFunc()
}

func genClassNewFunc(ctx *ctx, opt objectTypeBoilerplateOpt) {
	newFuncName := mangledNew(opt.class.Module, opt.class.Name)
	getType := qbeil.Var{
		Name:   mangledClassTypeGetter(ctx.module, opt.class.Name),
		Global: true,
	}
	ctx.il.Func(qbeil.Linkage{Type: qbeil.Export}, ctx.ptrType, "$"+newFuncName, []qbeil.TypedVar{})
	typeVar := ctx.il.TempVar(false)
	ctx.il.Call(&typeVar, ctx.ptrType /*GType*/, getType, []qbeil.ABITypedValue{})

	constructName := mangleName(mangleOpts{
		module: opt.class.Module,
		class:  opt.class.Name,
		name:   "construct",
	})
	construct := qbeil.Var{Global: true, Name: constructName}
	ret := ctx.il.TempVar(false)
	ctx.il.Call(&ret, ctx.ptrType, construct, []qbeil.ABITypedValue{
		{Type: ctx.ptrType, Value: typeVar},
	})

	ctx.il.Ret(ret)
	ctx.il.EndFunc()
}

func genInterfaceDef(ctx *ctx, e *parse.ObjectTypeDef) {
	// dummy type
	// interfaces are "opaque" and only passed around as pointers
	// TODO: should names be canonicalized?
	ctx.declareType(e.Name, qbeil.StructType{
		Align:   0,
		Name:    e.Name,
		Layouts: map[string]qbeil.FieldLayout{},
		Fields:  []qbeil.RepeatType{},
	})

	ct := ctx.simplify(e.ID())
	classTy, ok := ct.(*types.Class)
	if !ok {
		panic(fmt.Sprintf("compiler bug: class definition yields non-class type %#v", ct))
	}
	ifaceType := ctx.ensureObjectTypeDeclared(classTy)
	genObjectTypeBoilerplate(ctx, objectTypeBoilerplateOpt{
		class:             classTy,
		maybeInstanceType: nil,
		classType:         ifaceType,
		maybePrivate:      nil,
		iface:             true,
		extern:            e.Extern,
		parents: fun.Map(classTy.Supers, func(app *types.Application) *types.Class {
			return assert.Cast[*types.Class](ctx.typeApplicationToType(app),
				"codegen: super %s.%s is not a class", app.Module, app.Name)
		}),
	})
	genInterfaceMethodWrappers(ctx, e.Name, classTy, ifaceType)
}

// Generate functions that accept an object instance + args,
// retrieves the vtable of the instance and calls the method with the args
func genInterfaceMethodWrappers(
	ctx *ctx,
	ifaceName string,
	ct *types.Class,
	ifaceType qbeil.StructType,
) {
	for name, meth := range ct.Methods.All() {
		mangled := mangleName(mangleOpts{
			module: ctx.module,
			class:  ifaceName,
			name:   name,
		})
		wrapperVar := qbeil.Var{Name: mangled, Global: true}
		methTy := meth.Type.(*types.Func)
		retTy := methTy.Ret
		retIlTy := ctx.toABIType(retTy)
		args := make([]qbeil.TypedVar, 0, len(methTy.Args))
		args = append(args, qbeil.TypedVar{
			Type: ctx.ptrType,
			Name: qbeil.Var{Name: ctx.newTempName("self"), Global: false},
		})
		for _, arg := range methTy.Args {
			args = append(args, qbeil.NewTypedVar(
				ctx.toABIType(arg),
				ctx.il.TempVar(false),
			))
		}

		ctx.il.Func(qbeil.Linkage{}, retIlTy, wrapperVar.IL(), args)
		{
			argVals := fun.Map(args, func(v qbeil.TypedVar) qbeil.ABITypedValue {
				return qbeil.ABITypedValue{
					Type:  v.Type,
					Value: v.Name,
				}
			})

			self := argVals[0].Value

			// g_type_interface_peek(((GTypeInstance*) ip)->g_class, gt)
			// type_inst = ((GTypeInstance*) self)->g_class
			type_inst := genGetStructPtrField(ctx, self, ctx.gObject(), "g_class")

			// ifaceType := animal_get_type()
			ifaceTypeVar := qbeil.Var{Name: ctx.newTempName(ifaceName), Global: false}
			getType := qbeil.Var{Name: mangledClassTypeGetter(ctx.module, ifaceName), Global: true}
			ctx.il.Call(&ifaceTypeVar, ctx.ptrType, getType, []qbeil.ABITypedValue{})

			vtableInst := qbeil.Var{Global: false, Name: ctx.newTempName(ct.Name + "_" + ifaceName + "_vtable")}
			interfacePeek := qbeil.Var{Name: "g_type_interface_peek", Global: true}
			// vtableInst = g_type_interface_peek(type_inst, ifaceTypeVar)
			ctx.il.Call(&vtableInst, ctx.ptrType, interfacePeek, []qbeil.ABITypedValue{
				{Type: ctx.ptrType, Value: type_inst},
				{Type: ctx.ptrType, Value: ifaceTypeVar},
			})

			thenLabel := ctx.il.TempLabel("then_")
			elseLabel := ctx.il.TempLabel("else_")
			ctx.il.Jnz(vtableInst, thenLabel, elseLabel)
			{
				ctx.il.InsertLabel(thenLabel)
				methPtr := genGetStructPtrField(ctx, vtableInst, ifaceType, name)
				retVar := ctx.il.TempVar(false)
				ctx.il.Call(&retVar, retIlTy, methPtr, argVals)
				ctx.il.Ret(retVar)
			}

			ctx.il.InsertLabel(elseLabel)
			ctx.il.Ret(qbeil.IntLiteral{Value: 0})
		}
		ctx.il.EndFunc()
	}
}

func genGetStructPtrField(
	ctx *ctx,
	structPtr qbeil.Value,
	structTy qbeil.StructType,
	field string,
) qbeil.Var {
	addr := qbeil.Var{
		Name:   ctx.newTempName("ptrTo_" + structTy.Name + "." + field),
		Global: false,
	}
	fieldLayout := assert.Get(structTy.Layouts, field)
	ctx.il.Arithmetic(addr.IL(), ctx.ptrType, "add",
		structPtr,
		// FIXME: dividing 8 here looks extremely scuffed
		qbeil.IntLiteral{Value: int64(fieldLayout.OffsetBits / 8)},
	)

	return genReturnableValueFromPtr(ctx, addr, fieldLayout.Type)
}

func genSetStructPtrField(
	ctx *ctx,
	structPtr qbeil.Value,
	structTy qbeil.StructType,
	field string,
	value qbeil.Value,
) {
	addr := qbeil.Var{
		Name:   ctx.newTempName("ptrTo_" + structTy.Name + "." + field),
		Global: false,
	}
	fieldLayout := assert.Get(structTy.Layouts, field)
	ctx.il.Arithmetic(addr.IL(), ctx.ptrType, "add",
		structPtr,
		// FIXME: dividing 8 here looks extremely scuffed
		qbeil.IntLiteral{Value: int64(fieldLayout.OffsetBits / 8)},
	)

	bt, ok := fieldLayout.Type.(qbeil.BaseType)
	if !ok {
		panic("TODO: getting non-base-typed struct field")
	}

	ctx.il.Command("store"+bt.IL(), value, addr)
}

type typeInfoOpt struct {
	className    string
	classSize    uint16
	instanceSize uint16
}

// writes this block of C code:
//
//	static const GTypeInfo g_define_type_info = {
//		.class_size = sizeof (FooIface),
//
//		.base_init = (GBaseInitFunc) NULL,
//		.base_finalize = (GBaseFinalizeFunc) NULL,
//
//		/* interface types, classed types, instantiated types */
//		.class_init = foo_default_init,
//		.class_finalize = (GClassFinalizeFunc) NULL,
//		.class_data = NULL,
//
//		/* instantiated types */
//		.instance_size = 0,
//		.n_preallocs = 0,
//		.instance_init = (GInstanceInitFunc) NULL,
//
//		/* value handling */
//		.value_table = NULL
//	};
func genConstDefineTypeInfo(
	ctx *ctx,
	typeInfo typeInfoOpt,
) qbeil.Var {
	typeInfoName := mangleName(mangleOpts{
		module: ctx.module,
		class:  typeInfo.className,
		name:   "g_define_type_info",
	})
	return ctx.il.CompositeData(
		qbeil.DataDef{
			Linkage: qbeil.Linkage{},
			VarName: typeInfoName,
			Align:   0,
		},
		qbeil.DataItems([]qbeil.TypedDataItem{
			// class_size
			{Type: qbeil.HalfWord, Value: qbeil.IntLiteral{Value: int64(typeInfo.classSize)}},

			// manual padding lmao TODO: pad automatically
			{Type: qbeil.HalfWord, Value: qbeil.IntLiteral{Value: 0}},
			{Type: qbeil.Word, Value: qbeil.IntLiteral{Value: 0}},

			{Type: ctx.ptrType, Value: qbeil.IntLiteral{Value: 0}}, // base_init
			{Type: ctx.ptrType, Value: qbeil.IntLiteral{Value: 0}}, // base_finalize

			{Type: ctx.ptrType, Value: qbeil.IntLiteral{Value: 0}}, // class_init
			{Type: ctx.ptrType, Value: qbeil.IntLiteral{Value: 0}}, // class_finalize
			{Type: ctx.ptrType, Value: qbeil.IntLiteral{Value: 0}}, // class_data

			// should be (type)_class_init
			{Type: qbeil.HalfWord, Value: qbeil.IntLiteral{Value: int64(typeInfo.instanceSize)}}, // instance_size
			{Type: qbeil.HalfWord, Value: qbeil.IntLiteral{Value: 0}},                            // n_preallocs
			{Type: ctx.ptrType, Value: qbeil.IntLiteral{Value: 0}},                               // instance_init

			{Type: ctx.ptrType, Value: qbeil.IntLiteral{Value: 0}}, // value_table
		}),
	)
}

func genUnionDef(ctx *ctx, e *parse.UnionDef) {
	st := ctx.simplify(e.ID())
	unionTy, ok := st.(*types.Union)
	if !ok {
		panic(fmt.Sprintf("compiler bug: union definition yields non-class type %#v", st))
	}

	ctx.declareType(e.Name, ctx.unionDefIL(unionTy, e))
}

func genDotAccessOnAny(ctx *ctx, e *parse.RecordAccess) qbeil.Value {
	// supports:
	// 1. var1.field.moreFields
	// 2. Module.var1.field.moreFields
	// class static variables not supported
	if lhsSym, ok := e.Record.(*parse.Symbol); ok {
		if modName, ok := ctx.imports[lhsSym.Name]; ok {
			return qbeil.Var{
				Global: true,
				Name: mangleName(mangleOpts{
					module: modName,
					class:  "",
					name:   e.Field,
				}),
			}
		}
	}
	st := ctx.simplify(e.Record.ID())
	return genDotAccessOnExpr(ctx, st, e)
}

func genDotAccessOnExpr(ctx *ctx, lhsTy types.Type, e *parse.RecordAccess) qbeil.Value {
	switch lhsTy := ctx.resolveTypeApplications(lhsTy).(type) {
	case *types.Class:
		mod := lhsTy.Module
		if mod == "" {
			mod = ctx.module
		}
		return genClassAccessByName(ctx, mod, lhsTy.Name, e)
	case *types.Record:
		ilTy := ctx.toILType(lhsTy)

		structTy, ok := ilTy.(qbeil.StructType)
		assert.True(ok, fmt.Sprintf(
			"BUG: expected LHS to be a struct type but is a %T:\n  %v",
			ilTy, e.Record))
		lhs := gen(ctx, e.Record)
		return genGetStructPtrField(ctx, lhs, structTy, e.Field)

	default:
		panic(fmt.Sprintf("unexpected types.Type: %#v, at %v", lhsTy, e))
	}
}

func genClassAccessByName(ctx *ctx, module can.ModuleName, class string, expr *parse.RecordAccess) qbeil.Value {
	assert.Neq(class, "", "unnamed class in class access not yet supported")
	assert.Neq(module, "", "unnamed module of class", class)

	var classTy qbeil.StructType
	if module != ctx.module {
		ilTy := ctx.ensureClassNameDeclared(module, class)
		classTy = assert.Cast[qbeil.StructType](ilTy,
			fmt.Sprintf("BUG codegen: IL type of %s.%s is not a StructType, but a %T", module, class, ilTy))
	} else {
		ct, ok := ctx.userTypes[class]
		assert.True(ok, "codegen: record access on type with no declared IL type? class:",
			class, "from expr:", expr)
		classTy, ok = ct.(qbeil.StructType)
		assert.True(ok, "codegen: record access on non-struct type (type checker bug?)",
			ct, "from expr: ", expr)
	}

	return genGetStructPtrField(
		ctx,
		gen(ctx, expr.Record),
		classTy,
		expr.Field,
	)
}

// Alias to [genGetStructPtrField]
func genRecordAccess(ctx *ctx, lhs qbeil.Value, structTy qbeil.StructType, field string) qbeil.Var {
	return genGetStructPtrField(ctx, lhs, structTy, field)
}

type recordAssignment struct {
	name  string
	typ   qbeil.Type
	value qbeil.Value
}

func genRecordLiteral(ctx *ctx, ilTy qbeil.StructType, heap bool, fields []recordAssignment) qbeil.Value {
	structBits, _ := ctx.sizeOf(ilTy)
	assert.Neq(0, structBits,
		"BUG codegen: genRecordLiteral cannot be used on zero-sized records")

	rcdPtr := ctx.il.TempNamedVar(false, "record_literal_")

	if heap {
		mallocFunc := qbeil.Var{Global: true, Name: "calloc"}
		ctx.il.Call(&rcdPtr, ctx.ptrType, mallocFunc, []qbeil.ABITypedValue{
			{Type: ctx.ptrType, Value: qbeil.IntLiteral{Value: 1}},
			{Type: ctx.ptrType, Value: qbeil.IntLiteral{Value: int64(bitsToBytesRoundedUp(structBits))}},
		})
	} else {
		ctx.il.Arithmetic(rcdPtr.IL(), ctx.ptrType, "alloc4", qbeil.IntLiteral{Value: int64(structBits / 8)})
	}

	for _, field := range fields {
		layout := ilTy.Layouts[field.name]
		offsetBits := layout.OffsetBits

		fieldBitsFromLayout, _ := ctx.sizeOf(layout.Type)
		fieldBitsValType, _ := ctx.sizeOf(field.typ)

		assert.Eq(fieldBitsFromLayout, fieldBitsValType,
			fmt.Sprintf(
				"BUG record literal: field size different in layout and value, layout: %d, value: %d",
				fieldBitsFromLayout, fieldBitsValType,
			))

		fieldPtr := ctx.il.TempNamedVar(false, "record_literal_field_"+field.name)
		ctx.il.Arithmetic(fieldPtr.IL(), ctx.ptrType, "add", rcdPtr, qbeil.IntLiteral{Value: int64(offsetBits / 8)})

		genCopyToPtr(ctx, fieldPtr, field.typ, field.value)
	}

	return rcdPtr
}

func genCopyToPtr(ctx *ctx, fieldPtr qbeil.Value, ilTy qbeil.Type, src qbeil.Value) {
	fieldBits, _ := ctx.sizeOf(ilTy)
	switch ilTy1 := ilTy.(type) {
	case qbeil.BaseType:
		ctx.il.Command("store"+ilTy1.IL(), src, fieldPtr)
	case qbeil.ExtraType:
		ctx.il.Command("store"+ilTy1.IL(), src, fieldPtr)
	case qbeil.AggregateType: // struct or union type
		// TODO: gen returns a pointer right?
		// TODO: memcpy is preferred for large sized copies
		bytes := int64(bitsToBytesRoundedUp(fieldBits))
		ctx.il.Command("blit", src, fieldPtr, qbeil.IntLiteral{Value: bytes})
	default:
		panic(fmt.Sprintf("unexpected IL type: %v", ilTy))
	}
}

// Takes a pointer,
//
//   - if it points to a [qbeil.BaseType] or [qbeil.ExtraType] dereference and
//     return it
//   - if it points to a compound type (struct/union), return the pointer untouched
func genReturnableValueFromPtr(ctx *ctx, ptr qbeil.Var, ilTy qbeil.Type) qbeil.Var {
	switch bt := ilTy.(type) {
	case qbeil.BaseType:
		val := ctx.il.TempNamedVar(false, "dataAt_"+ptr.Name)
		ctx.il.Arithmetic(val.IL(), bt, "load"+ilTy.IL(), ptr)
		return val
	case qbeil.ExtraType:
		switch bt {
		case qbeil.Byte, qbeil.HalfWord:
			val := ctx.il.TempNamedVar(false, "temp")
			ctx.il.Arithmetic(val.IL(), qbeil.Word, "loadw", ptr)
			return val
		default:
			panic(fmt.Sprintf("unexpected qbeil.ExtraType: %#v", bt))
		}
	case qbeil.StructType, qbeil.UnionType:
		return ptr
	default:
		panic(fmt.Sprintf("unexpected qbeil.Type: %#v", bt))
	}
}

func genUnrefAndPopScope(ctx *ctx, comment string) {
	// FIXME: take ownership into account
	scope := ctx.vars.PopScope()
	for val := range scope.Values() {
		genUnref(ctx, qbeil.Var{
			Global: false,
			Name:   val.uname,
		}, val.typ, fmt.Sprintf("%s.%s_%s", ctx.module, comment, val.uname))
	}
}

func genRef(ctx *ctx, v qbeil.Value, ty types.Type, comment string) {
	switch ty.(type) {
	case *types.Class:
		if comment != "" {
			ctx.il.Comment("%s", comment)
			// commentVar := qbeil.Var{Global: true, Name: debugMangle(comment)}
			// ctx.statics[commentVar] = fmt.Sprintf(`{ b "%s\n", b 0 }`, "ref: "+comment)
			// ctx.il.Call(nil, nil, qbeil.Var{Global: true, Name: "printf"}, []qbeil.ABITypedValue{
			// 	{Type: ctx.ptrType, Value: commentVar},
			// })
		}
		ref := qbeil.Var{Global: true, Name: "g_object_ref"}
		ctx.il.Call(nil, nil, ref, []qbeil.ABITypedValue{
			{Type: ctx.ptrType, Value: v},
		})
	default:
	}
}

func genUnref(ctx *ctx, v qbeil.Var, ty types.Type, comment string) {
	ty = ctx.resolveTypeApplications(ty)
	switch ty.(type) {
	case *types.Class:
		if comment != "" {
			ctx.il.Comment("%s", comment)
			// commentVar := qbeil.Var{Global: true, Name: debugMangle(comment)}
			// ctx.statics[commentVar] = fmt.Sprintf(`{ b "%s", b 0 }`, comment)
		}

		unref := qbeil.Var{Global: true, Name: "g_object_unref"}
		ctx.il.Call(nil, nil, unref, []qbeil.ABITypedValue{
			{Type: ctx.ptrType, Value: v},
			// {Type: ctx.ptrType, Value: commentVar},
		})
	default:
	}
}

func (ctx *ctx) finish() error {
	if _, err := ctx.il.OutFile.Write(ctx.typeDecl.Bytes()); err != nil {
		return err
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

	_, err := io.Copy(ctx.il.OutFile, &ctx.il.Buf)
	return err
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
		if t.Fields.Len() == 0 {
			// HACK: massive hack, I really should make empty records zero
			// width, but that would mean rewriting a lot of things so that
			// function calls don't have to return something.
			// This is also completely wrong ABI-wise (returning an empty
			// record should mean no return value in C ABI)
			return ctx.ptrType
		}
		return ctx.recordToILType(t)

	case *types.Func:
		return assert.Get(ctx.userTypes, "ClosureComponents",
			"BUG codegen: ClosureComponents not declared?")

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
		name := ctx.newTempName("_array")
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
		return assert.Get(ctx.userTypes, "List",
			"BUG codegen: List struct type not defined?")

	case *types.TaggedUnion:
		ty, _ := ctx.taggedUnionToILType(t)
		return ty

	case *types.ExplicitOwnership:
		return ctx.toILType(t.Content)

	default:
		panic("unimpl: conversion to QBE IL from type " + typ.String())
	}
}

// used in function arg type and return type, see qbe docs
// TODO: explain difference to [ctx.toILType]
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
	assert.Neq(0, t.Fields.Len(),
		"BUG codegen: recordToILType does not accept empty record")
	if il, ok := ctx.generatedTranslation[t]; ok {
		return il.(qbeil.StructType)
	}

	fields := make([]qbeil.RepeatType, 0, t.Fields.Len())
	layouts := map[string]qbeil.FieldLayout{}
	offset := 0

	for name, fieldTy := range t.Fields.All() {
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

func (ctx *ctx) taggedUnionToILType(t *types.TaggedUnion) (ty qbeil.StructType, payload qbeil.UnionType) {
	if il, ok := ctx.generatedTranslation[t]; ok {
		ty := assert.Cast[qbeil.StructType](il, "BUG cached tagged union IL type is not StructType?")
		layout := assert.Get(ty.Layouts, "payload", "Tagged union type has no field payload?")
		payload := assert.Cast[qbeil.UnionType](layout.Type, "Tagged union payload is not a UnionType?")
		return ty, payload
	}

	name := t.Name
	if name == "" {
		name = ctx.newTempName("TaggedUnion")
	}
	variants := slices.Collect(fun.MapFilter(t.Variants.Values(), func(o opt.Option[types.Type]) opt.Option[qbeil.Type] {
		if ty, ok := o.Unwrap(); ok {
			return opt.Some(ctx.toILType(ty))
		}
		return opt.None[qbeil.Type]()
	}))

	size := 0
	for _, v := range variants {
		bits, _ := ctx.sizeOf(v)
		size = max(size, (bits))
	}

	payloadUnion := qbeil.UnionType{
		Name:     name + "_Payload",
		Align:    0,
		Size:     size,
		Variants: variants,
	}
	ctx.declareType(name+"_Payload", payloadUnion)

	ptrBits, _ := qbeil.SizeOf(0, ctx.ptrType)
	ty = qbeil.StructType{
		Align: 0,
		Name:  name,
		Layouts: map[string]qbeil.FieldLayout{
			"tag":     {Type: ctx.ptrType, OffsetBits: 0},
			"payload": {Type: payloadUnion, OffsetBits: ptrBits},
		},
		Fields: []qbeil.RepeatType{
			qbeil.SingleType(ctx.ptrType),
			qbeil.SingleType(payloadUnion),
		},
	}
	ctx.declareType(name, ty)
	return ty, payloadUnion
}

// ensures a class has a declared IL type, useful for imported types, which
// we don't always want to include in the IL
func (ctx *ctx) ensureClassDeclared(t *types.Class) qbeil.AggregateType {
	if t.Module == ctx.module {
		if ilTy, ok := ctx.userTypes[t.Name]; ok {
			return ilTy
		}
		panic(fmt.Sprintf("BUG local class %s.%s undeclared during codegen", t.Module, t.Name))
	}

	ilTy := ctx.classToILType(t)
	ilTyName := string(t.Module) + "." + t.Name
	if _, ok := ctx.userTypes[ilTyName]; !ok {
		ctx.declareType(ilTyName, ilTy)
	}

	return ilTy
}

// like [ctx.ensureClassDeclared], but takes the type name
func (ctx *ctx) ensureClassNameDeclared(mod can.ModuleName, class string) qbeil.AggregateType {
	if mod == ctx.module {
		if ilTy, ok := ctx.userTypes[class]; ok {
			return ilTy
		}
		panic(fmt.Sprintf("BUG local class %s.%s undeclared during codegen", mod, class))
	}

	ilTyName := string(mod) + "." + class
	if ilTy, ok := ctx.userTypes[ilTyName]; ok {
		return ilTy
	}

	module := assert.Get(ctx.allModules, mod,
		"BUG codegen: encountered unresolved module", mod)
	ts := assert.Get(module.Types, class,
		"BUG unresolved imported type", mod, ".", class, "encountered during codegen")
	st := assert.Cast[simplesub.SimpleType](ts,
		"BUG codegen: encountered type scheme", mod, ".", class)
	ty := assert.Cast[*types.Class](ctx.simplifyType(st),
		"BUG codegen: encountered non-class type where a class is expected", mod, ".", class)

	return ctx.ensureClassDeclared(ty)
}

func (ctx *ctx) ensureObjectTypeDeclared(t *types.Class) qbeil.StructType {
	ilTyName := ilTypeName(t.Module, t.Name+"Class")
	if tyClass, ok := ctx.userTypes[ilTyName]; ok {
		return assert.Cast[qbeil.StructType](tyClass, "codegen: class/iface type is not a StructType?")
	}

	parentClass := assert.Get(ctx.userTypes, "GObjectClass", "undefined class type GObjectClass?")
	for _, super := range t.Supers {
		ty := assert.Cast[*types.Class](ctx.typeApplicationToType(super),
			fmt.Sprintf("codegen: a super class of %s is not a class: %s.%s",
				t.Name, super.Module, super.Name))
		if ty.Kind == parse.Class {
			parentClass = ctx.ensureObjectTypeDeclared(ty)
			// TODO: handle multiple class supers?
			break
		}
	}

	// virtual method pointers
	// TODO: only include virtual methods
	bits, _ := ctx.sizeOf(parentClass)
	vtableOffset := bits
	fields := []qbeil.RepeatType{qbeil.SingleType(parentClass)}
	layouts := map[string]qbeil.FieldLayout{}
	ptrBits, _ := ctx.sizeOf(ctx.ptrType)
	for _, meth := range t.Methods.Keys() {
		fields = append(fields, qbeil.SingleType(ctx.ptrType))
		layouts[meth] = qbeil.FieldLayout{
			Type:       ctx.ptrType,
			OffsetBits: vtableOffset,
		}
		vtableOffset += ptrBits
	}

	ct := qbeil.StructType{
		Name:    ilTyName,
		Fields:  fields,
		Align:   0,
		Layouts: layouts,
	}
	ctx.declareType(ilTyName, ct)

	return ct
}

// Recursively resolves *types.Application
func (ctx *ctx) typeApplicationToType(app *types.Application) types.Type {
	module := assert.Get(ctx.allModules, app.Module,
		"BUG codegen: encountered unresolved module", app.Module)
	ts := assert.Get(module.Types, app.Name,
		"BUG unresolved imported type", app.Module, ".", app.Name, "encountered during codegen")
	st := assert.Cast[simplesub.SimpleType](ts,
		"BUG codegen: encountered type scheme", app.Module, ".", app.Name)
	ty := ctx.simplifyType(st)
	if app, ok := ty.(*types.Application); ok {
		return ctx.typeApplicationToType(app)
	}
	return ty
}

func (ctx *ctx) resolveTypeApplications(t types.Type) types.Type {
	if app, ok := t.(*types.Application); ok {
		return ctx.typeApplicationToType(app)
	}
	return t
}

func ilTypeName(module can.ModuleName, class string) string {
	return string(module) + "." + class
}

// like [ctx.toILType] but converts class type to its full [qbeil.AggregateType] instead of a pointer type
func (ctx *ctx) classToILType(t *types.Class) qbeil.AggregateType {
	if t.Name == "" {
		// FIXME: generic support
		panic("unnamed classes should be illegal at codegen")
	}
	assert.Eq(t.Kind, parse.Class, "BUG: classToILType called on an interface?")

	ilName := t.Name
	if t.Module != ctx.module {
		ilName = string(t.Module) + "." + t.Name
	}

	if ut, ok := ctx.userTypes[ilName]; ok {
		return ut
	}

	var classParent qbeil.AggregateType
	if len(t.Supers) != 0 {
		// TODO: assert super is class or something
		// TODO: multi inheritence
		sup := t.Supers[0]
		supTy := ctx.typeApplicationToType(t.Supers[0])
		supClass := assert.Cast[*types.Class](supTy,
			"BUG encountered non-class super type", sup.Module, ".", sup.Name, "during codegen")
		classParent = ctx.ensureClassDeclared(supClass) // TODO
	} else if !t.Top {
		classParent = assert.Get(ctx.userTypes, "GObject", "BUG type Object not defined?")
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

func (ctx *ctx) gObject() qbeil.StructType {
	return assert.Cast[qbeil.StructType](
		assert.Get(ctx.userTypes, "GObject", "genqbe: GObject not declared?"),
		"genqbe: GObject is not StructType?",
	)
}

func (ctx *ctx) gTypeClass() qbeil.StructType {
	return assert.Cast[qbeil.StructType](
		assert.Get(ctx.userTypes, "GTypeClass", "genqbe: GTypeClass not declared?"),
		"genqbe: GTypeClass is not StructType?",
	)
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

func tagValue(u *types.TaggedUnion, tag string) int64 {
	i := int64(slices.Index(u.Variants.Keys(), tag))
	assert.Neq(i, -1, fmt.Sprintf(
		"Tag '%s' not found in tag union: %s", tag, u.String()))
	return i
}

func flattenModuleAccessPath(expr *parse.RecordAccess) can.ModuleName {
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

func bitSizeToIntType(bits int) (_ qbeil.Type, ok bool) {
	switch bits {
	case 8:
		return qbeil.Byte, true
	case 16:
		return qbeil.HalfWord, true
	case 32:
		return qbeil.Word, true
	case 64:
		return qbeil.Long, true
	}
	return nil, false
}

func callerName() string {
	pc, _, _, _ := runtime.Caller(1)
	return runtime.FuncForPC(pc).Name()
}

func bitsToBytesRoundedUp(bits int) int {
	return (bits + 7) / 8
}

func mapHas[K comparable, V any](m map[K]V, key K) bool {
	_, ok := m[key]
	return ok
}

func popSlice[V any](s *[]V) opt.Option[V] {
	if len(*s) == 0 {
		return opt.None[V]()
	}
	last := (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]
	return opt.Some(last)
}

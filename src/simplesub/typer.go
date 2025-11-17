package simplesub

import (
	"errors"
	"fmt"
	"strings"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/can"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/internal/scope"
	"github.com/horriblename/typee/src/opt"
	"github.com/horriblename/typee/src/parse"
)

type symbols struct {
	vars     scope.ScopedMap[TypeScheme]
	types    scope.ScopedMap[TypeScheme]
	inferred map[int]TypeScheme
	imports  map[string]ModuleInfo

	// shared across all modules
	moduleCache map[can.ModuleName]ModuleInfo
	mainModule  can.ModuleName
}

type Typer struct {
	symbols

	debug      bool
	classScope string
}

var ErrUndefinedVariable = errors.New("undefined variable")
var ErrUndefinedTypeName = errors.New("undefined type")
var ErrUndefinedModule = errors.New("undefined module. missing import?")
var ErrSelfUnbound = errors.New("keyword self used outside of a method")
var ErrSelfTypeUnbound = errors.New("keyword Self used outside of a class definition")
var ErrWrongArgCount = errors.New("wrong argument count")
var ErrTypeMismatch = errors.New("mismatched type")
var ErrMissingField = errors.New("missing field")
var ErrMissingMethod = errors.New("missing method")
var ErrMissingStaticMethod = errors.New("missing static method")
var ErrConstraintViolated = errors.New("constraint violated")
var ErrCannotConstrain = errors.New("cannot constrain")
var ErrInvalidTopLevel = errors.New("invalid top level construct: must be set or def")
var ErrDefMustBeTopLevel = errors.New("function definitions only allowed in top level")
var ErrClassDefMustBeTopLevel = errors.New("class and interface definitions must be top-level")
var ErrEmptyFuncBody = errors.New("empty function body")
var ErrUnionUnexpectedVariant = errors.New("union type has unexpected variant")
var ErrIllegalTypeScheme = errors.New("type schemes not allowed here")
var ErrIllegalSuperType = errors.New("super types must be class or interfaces")
var ErrBadTypeSignature = errors.New("bad function type signature")
var ErrPolymorphicUnion = errors.New("generics are disallowed from binding to unions")
var ErrNotEnum = errors.New("tried to use non-enum as enum")
var ErrWrongEnumType = errors.New("enum types do not match")
var ErrWrongUnionType = errors.New("union types do not match")
var ErrUseRecordAsNamedObjectType = errors.New("cannot use a record as a named object type")
var ErrUseNamedObjectTypeAsRecord = errors.New("cannot use a named object type as a record")
var ErrIllegalPolymorphicType = errors.New("illegal use of polymorphic type")
var ErrUnparameterizedTypePassedParams = errors.New("an unparameterized type was passed type parameters")
var ErrEnumMissingKey = errors.New("enum type is missing a key")
var ErrMethodMissingSignature = errors.New("method must have signature")
var ErrPolymorphicInSignature = errors.New("illegal polymorphic type in function signature")
var ErrMethodCallOnStaticMethod = errors.New("tried to call static method on object instance")
var ErrStaticMethodCallOnNonStatic = errors.New("tried to call method as a static method")
var ErrInvalidClassCoercion = errors.New("invalid class coercion")
var ErrIncompleteTypeInSignature = errors.New("incomplete type used in signature")

const scopeLevelTop int = 1

func NewTyper(mainModule can.ModuleName, debug bool) *Typer {
	vars := scope.ScopedMapWithGlobals(builtinVars())
	vars.NewScope()
	types := scope.ScopedMapWithGlobals(builtinTypes())
	types.NewScope()
	stdMod := ModuleInfo{
		Name:     StdModName,
		Ast:      []parse.Expr{},
		TypesAst: []parse.Expr{},
		Types: map[string]TypeScheme{
			"Object": gobject,
		},
		Globals:  map[string]TypeScheme{},
		TypeTree: map[int]TypeScheme{},
	}
	return &Typer{
		symbols: symbols{
			vars:     vars,
			types:    types,
			inferred: map[int]TypeScheme{},
			imports: map[string]ModuleInfo{
				string(StdModName): stdMod,
			},
			moduleCache: map[can.ModuleName]ModuleInfo{
				StdModName: stdMod,
			},
			mainModule: mainModule,
		},
		debug:      debug,
		classScope: "",
	}
}

func (self *Typer) TypeProgram(program []parse.Expr) ([]TypeScheme, map[can.ModuleName]ModuleInfo, error) {
	if err := self.typeDeps(program); err != nil {
		return nil, nil, err
	}

	self.inferred = map[int]TypeScheme{}

	t, typesAst, err := self.typeProgram(program)
	if err != nil {
		return nil, nil, err
	}

	self.moduleCache[self.mainModule], err = typeTableToSymbolMap(self.mainModule, program, typesAst, self.inferred)
	if err != nil {
		return nil, nil, err
	}

	return t, self.moduleCache, err
}

func (self *Typer) typeProgram(program []parse.Expr) (_ []TypeScheme, typesAst []parse.Expr, _ error) {
	// top-level process
	// 1. type check imported modules (already done in [TypeProgram])
	// 2. groupRecursives: walk the AST to mark (mutually-)recursive top-level functions.
	// 3. sort type definitions by dependency (cyclic types not currently supported)
	// 4. process type definitions
	// 5. iterate through top-level nodes generating fresh type vars for each top level non-type-def node.
	// 6. walk the AST, inferring types of all expressions
	importCount := 0
	// FIXME: kinda confusing where to reset imports (and other per-module state)
	self.imports = map[string]ModuleInfo{
		string(StdModName): assert.Get(self.moduleCache, StdModName,
			"BUG Std missing; moduleCache not set up correctly?"),
	}
	for i, expr := range program {
		expr, ok := expr.(*parse.Import)
		if !ok {
			importCount = i
			break
		}

		alias := expr.Module[len(expr.Module)-1]
		fullPath := can.ModuleName(strings.Join(expr.Module, "."))
		self.imports[alias], ok = self.moduleCache[fullPath]
		assert.True(ok, "typer bug: module", fullPath, "missing from moduleCache")
		assert.Eq(self.imports[alias].Name, fullPath, "import-aliased module has wrong full path?")
	}

	program = program[importCount:]

	types := make([]TypeScheme, len(program))

	topLevels := map[string]parse.Expr{}

	recursiveness := map[string]bool{}
	groups, selfRecursives := groupRecursives(program)

	typeDefOrder, typeDefAsts, _ := sortTypeDefs(program)

	// TODO: do I still need to sort type defs since I started using Application types?

	// process type definitions
	for _, group := range typeDefOrder {
		if len(group) == 1 {
			if _, ok := typeDefAsts[group[0]]; !ok {
				// skip external (?) type defs
				continue
			}
		}

		for _, name := range group {
			ast, ok := typeDefAsts[name]
			assert.True(ok, "compiler bug: ast lookup failed: ", name)
			t, err := self.defType(ast)
			if err != nil {
				return nil, nil, fmt.Errorf("defining type %s: %w", name, err)
			}

			self.types.Insert(name, PolymorphicType{Body: t})
		}
	}

	// could be optimized but eh
	for _, group := range groups {
		if len(group) == 1 {
			name := group[0]
			if _, ok := selfRecursives[name]; !ok {
				recursiveness[name] = false
				continue
			}
		}

		for _, name := range group {
			recursiveness[name] = true
		}
	}

	// assign fresh type vars to each top level value
	for i, expr := range program {
		switch e := expr.(type) {
		case *parse.FuncDef:
			// recursive (inluding mutual-recursive) functions are not generalized
			if !recursiveness[e.Name] {
				types[i] = PolymorphicType{Body: freshVar()}
			} else {
				types[i] = freshVar()
			}
			self.vars.Insert(e.Name, types[i])
			topLevels[e.Name] = e

		case *parse.Set:
			if !recursiveness[e.Name] {
				types[i] = PolymorphicType{Body: freshVar()}
			} else {
				types[i] = freshVar()
			}
			self.vars.Insert(e.Name, types[i])
			topLevels[e.Name] = e

		case *parse.ObjectTypeDef:
			types[i] = self.inferred[e.ID()]
		case *parse.UnionDef:
			types[i] = self.inferred[e.ID()]
		case *parse.EnumDef:
			types[i] = self.inferred[e.ID()]
		case *parse.TypeAlias:
			types[i] = self.inferred[e.ID()]

		default:
			return nil, nil, fmt.Errorf("%w:\n    %s", ErrInvalidTopLevel, expr.Pretty())
		}
	}

	// actually type expressions
	for i, expr := range program {
		switch e := expr.(type) {
		case *parse.FuncDef:
			if e.Extern {
				typ, err := self.externDef(e)
				if err != nil {
					return nil, nil, err
				}

				fnTy, ok := types[i].(SimpleType)
				if !ok {
					pt := assert.Cast[PolymorphicType](types[i], "cannot fail")
					fnTy = pt.Body
				}
				if err := self.symbols.constrain(typ, fnTy); err != nil {
					return nil, nil, fmt.Errorf("typing function %s: %w", e.Name, err)
				}

				self.inferred[e.ID()] = fnTy

				continue
			}

			if len(e.Body) == 0 {
				return nil, nil, fmt.Errorf("in function %s: %w", e.Name, ErrEmptyFuncBody)
			}

			fn := parse.Fn{
				Id:        expr.ID(),
				Signature: e.Signature,
				Args:      e.Args,
				Body:      e.Body[len(e.Body)-1],
			}
			typ, err := self.TypeTerm(&fn)
			if err != nil {
				return nil, nil, fmt.Errorf("in function %s: %w", e.Name, err)
			}

			placeholderTy, ok := types[i].(SimpleType)
			if !ok {
				pt := assert.Cast[PolymorphicType](types[i], "cannot fail")
				placeholderTy = pt.Body
			}
			if err := self.symbols.constrain(typ, placeholderTy); err != nil {
				return nil, nil, fmt.Errorf("verifying type signature of function %s: %w", e.Name, err)
			}

		case *parse.Set:
			typ, err := self.TypeTerm(e.Value)
			if err != nil {
				return nil, nil, err
			}

			valTy, ok := types[i].(SimpleType)
			if !ok {
				pt := assert.Cast[PolymorphicType](types[i], "cannot fail")
				valTy = pt.Body
			}

			if err := self.symbols.constrain(typ, valTy); err != nil {
				return nil, nil, err
			}

		case *parse.ObjectTypeDef:
			if err := self.defClassMethods(e); err != nil {
				return nil, nil, err
			}

		case *parse.UnionDef:
		case *parse.EnumDef:
		case *parse.TypeAlias:

		default:
			return nil, nil, fmt.Errorf("%w:\n    %s", ErrInvalidTopLevel, expr.Pretty())
		}
	}

	for _, group := range typeDefOrder {
		for _, name := range group {
			// builtins don't have an AST lookup, skip them
			if ast, ok := typeDefAsts[name]; ok {
				typesAst = append(typesAst, ast)
			}
		}
	}

	return types, typesAst, nil
}

func (self *Typer) TypeTerm(term parse.Expr) (a SimpleType, _ error) {
	trace("typing: %v", term.Pretty())
	indentLvl++
	defer func() {
		indentLvl--
		trace(": %v", a)
	}()
	defer func() {
		self.inferred[term.ID()] = a
	}()
	switch expr := term.(type) {
	case *parse.Symbol:
		if ty, ok := self.vars.Get(expr.Name).Unwrap(); ok {
			return ty.instantiate(), nil
		} else if mod, ok := self.imports[expr.Name]; ok {
			// FIXME: this is a terrible idea
			fields := make([]NamedType, 0, len(mod.Globals))
			for name, ty := range mod.Globals {
				fields = append(fields, NamedType{name, ty.instantiate()})
			}
			return Record{fields}, nil
		}
		return nil, fmt.Errorf("%w: %s", ErrUndefinedVariable, expr.Name)
	case *parse.SelfLiteral:
		if ty, err := self.parseType(parse.SelfType{}); err == nil {
			// TODO: this feels wrong, should'nt need to instantiate for every self right?
			// I should keep track of what type Self parses into, if it's a PolymorphicType
			// (or Application of PolymorphicType), I might be in trouble
			return ty.instantiate(), nil
		} else {
			return nil, err
		}
	case *parse.FuncDef:
		return nil, fmt.Errorf("%w: %s", ErrDefMustBeTopLevel, expr.Name)

	case *parse.ObjectTypeDef:
		return nil, fmt.Errorf("%w: %s", ErrClassDefMustBeTopLevel, expr.Name)

	case *parse.Fn:
		self.vars.NewScope()
		defer self.vars.PopScope()

		var err error
		if sig, ok := expr.Signature.Unwrap(); ok {
			if len(sig) != len(expr.Args)+1 {
				return nil, fmt.Errorf("%w: function has %d args but type signature only takes %d", ErrBadTypeSignature, len(expr.Args), len(sig)-1)
			}
			sigTy, err := self.parseFuncSignature(sig)
			if err != nil {
				return nil, err
			}

			argOffset := If(sigTy.Method, 1).Else(0)
			for i, p := range sigTy.Args {
				self.vars.Insert(expr.Args[i+argOffset], p)
			}

			bodyTy, err := self.TypeTerm(expr.Body)
			if err != nil {
				return nil, err
			}

			if err := self.symbols.constrain(sigTy.Ret, bodyTy); err != nil {
				return nil, err
			}

			// can probably just return sigTy?
			return Func{Args: sigTy.Args, Ret: sigTy.Ret, Method: sigTy.Method}, nil
		}

		paramsTy := make([]SimpleType, 0, len(expr.Args))
		for i, arg := range expr.Args {
			if i == 0 && arg == "self" {
				// TODO: don't think I need this
				ty, err := self.parseType(parse.SelfType{})
				if err != nil {
					return nil, err
				}

				paramsTy = append(paramsTy, ty.instantiate())
				continue
			}
			param := freshVar()
			paramsTy = append(paramsTy, param)
			self.vars.Insert(arg, param)
		}

		bodyTy, err := self.TypeTerm(expr.Body)
		if err != nil {
			return nil, err
		}

		return Func{Args: paramsTy, Ret: bodyTy}, nil

	case *parse.Form:
		return self.typeFunctionCall(expr)

	case *parse.New:
		class, err := self.lookupTypeName(expr.Class)
		if err != nil {
			return nil, err
		}
		return Func{[]SimpleType{}, class.instantiate(), false}, nil

	case *parse.BoolLiteral:
		return Primitive{PrimitiveBool}, nil
	case *parse.IntLiteral:
		return I64, nil
	case *parse.FloatLiteral:
		return Primitive{PrimitiveFloat}, nil
	case *parse.StrLiteral:
		return Str{}, nil
	case *parse.ArrayLiteral:
		elTyps := make([]SimpleType, len(expr.Elements))
		for i, el := range expr.Elements {
			elTy, err := self.TypeTerm(el)
			if err != nil {
				return nil, err
			}

			elTyps[i] = elTy
		}

		ty := freshVar()
		for _, elTy := range elTyps {
			if err := self.symbols.constrain(elTy, ty); err != nil {
				return nil, fmt.Errorf("array element has incompatible type with other elements before it: %w", err)
			}
		}

		return ArrayType{ty, uint(len(expr.Elements))}, nil

	case *parse.Record:
		fields := make([]NamedType, len(expr.Fields))
		for i, field := range expr.Fields {
			fieldTy, err := self.TypeTerm(field.Value)
			if err != nil {
				return nil, err
			}
			fields[i] = NamedType{
				Name: field.Name,
				Type: fieldTy,
			}
		}

		return Record{Fields: fields}, nil

	case *parse.RecordAccess:
		return self.typeRecordAccess(expr)

	case *parse.MethodAccess:
		panic("illegal use of method access, methods must be called: " + expr.Pretty())

	case *parse.EnumAccess:
		// FIXME: uh, actually type check this pls
		enum, err := self.lookupTypeName(expr.Enum)
		if err != nil {
			return nil, err
		}
		enumTy := enum.instantiate()

		bound := Enum{
			Name: "",
			Values: map[string]opt.Option[int64]{
				expr.Key: opt.None[int64](),
			},
		}

		if err := self.symbols.constrain(enumTy, bound); err != nil {
			return nil, err
		}

		return enumTy, nil

	case *parse.IfExpr:
		condTy, err := self.TypeTerm(expr.Condition)
		if err != nil {
			return nil, err
		}

		if err := self.symbols.constrain(condTy, Primitive{PrimitiveBool}); err != nil {
			return nil, err
		}

		retTy := freshVar()
		thenTy, err := self.TypeTerm(expr.Consequence)
		if err != nil {
			return nil, err
		}

		elseTy, err := self.TypeTerm(expr.Alternative)
		if err != nil {
			return nil, err
		}

		if err := self.symbols.constrain(thenTy, retTy); err != nil {
			return nil, err
		}

		if err := self.symbols.constrain(elseTy, retTy); err != nil {
			return nil, err
		}
		return retTy, nil

	case *parse.LetExpr:
		self.vars.NewScope()
		defer self.vars.PopScope()
		for _, ass := range expr.Assignments {
			ty, err := self.TypeTerm(ass.Value)
			if err != nil {
				return nil, err
			}

			self.vars.Insert(ass.Var, ty)
		}

		return self.TypeTerm(expr.Body)

	case *parse.CaseExpr:
	case *parse.Set:
		varTy, ok := self.vars.Get(expr.Name).Unwrap()
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUndefinedVariable, expr.Name)
		}

		val, err := self.TypeTerm(expr.Value)
		if err != nil {
			return nil, err
		}

		varSTy, ok := varTy.(SimpleType)
		if !ok {
			panic("TODO: call set on PolymorphicType")
		}

		err = self.symbols.constrain(val, varSTy)
		if err != nil {
			return nil, err
		}

		return Record{[]NamedType{}}, err

	case *parse.VarDef:
		// TODO: should var be a let rec?
		val, err := self.TypeTerm(expr.Value)
		if err != nil {
			return nil, err
		}

		self.vars.Insert(expr.Name, val)
		return Record{[]NamedType{}}, err

	case *parse.TaggedExpr:
	case *parse.ExternCall:
		for _, arg := range expr.Args {
			if _, err := self.TypeTerm(arg); err != nil {
				return nil, err
			}
		}

		return freshVar(), nil

	default:
		panic(fmt.Sprintf("unexpected parse.Expr: %#v", expr))
	}
	panic(fmt.Sprintf("unhandled: TypeTerm(%s)", term.Pretty()))
}

func (self *Typer) externDef(e *parse.FuncDef) (SimpleType, error) {
	self.vars.NewScope()
	defer self.vars.PopScope()

	sig, ok := e.Signature.Unwrap()
	assert.True(ok, "got extern declaration with no signature")

	params := make([]SimpleType, len(sig)-1)

	if len(sig) != len(e.Args)+1 {
		return nil, fmt.Errorf(
			"%w: function %s has %d args but type signature only takes %d",
			ErrBadTypeSignature, e.Name, len(e.Args), len(sig)-1)
	}

	for i, arg := range sig[:len(sig)-1] {
		param, err := self.parseType(arg)
		if err != nil {
			return nil, fmt.Errorf(
				"reading type of argument '%s' of function %s: %w",
				arg, e.Name, err)
		}

		p := param.instantiate()
		params[i] = p
		self.vars.Insert(e.Args[i], p)
	}

	retHint, err := self.parseType(sig[len(sig)-1])
	if err != nil {
		return nil, fmt.Errorf(
			"reading return type '%s' of function %s: %w",
			sig[len(sig)-1], e.Name, err)
	}

	return Func{Args: params, Ret: retHint.instantiate()}, nil
}

func (self *Typer) defType(def parse.Expr) (SimpleType, error) {
	switch d := def.(type) {
	// TODO: this shouldn't be possible (enum defs cannot depend on other types currently)
	// should I assert against this?
	case *parse.EnumDef:
		typ, err := self.parseEnumDef(d)
		if err != nil {
			return nil, err
		}

		self.inferred[d.ID()] = typ
		return typ, nil

	case *parse.UnionDef:
		t, err := self.parseUnionDef(d)
		if err != nil {
			return nil, err
		}

		self.inferred[d.ID()] = t
		return t, nil

	case *parse.ObjectTypeDef:
		t, err := self.parseClassOutline(d)
		if err != nil {
			return nil, err
		}

		// TODO: handle generics
		self.inferred[d.ID()] = t
		return t, nil

	case *parse.TypeAlias:
		target, err := self.parseType(d.Type)
		if err != nil {
			return nil, err
		}

		self.inferred[d.ID()] = target

		// FIXME: should not instantiate!
		// this works cuz we don't have generalized type alias currenlty
		return target.instantiate(), nil
	default:
		panic(fmt.Sprintf("compiler bug: defType called with not type definition %#v", def))
	}
}

// type checks fields and assigns fresh variables to methods
func (self *Typer) parseClassOutline(classDef *parse.ObjectTypeDef) (SimpleType, error) {
	self.classScope = classDef.Name
	defer func() { self.classScope = "" }()

	supers := make([]Application, len(classDef.Supers))
	for i, s := range classDef.Supers {
		// TODO: use parseType to "resolve" type names
		modName := self.mainModule
		if s.Module != "" {
			mod, ok := self.imports[s.Module]
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrUndefinedModule, s.Module)
			}
			modName = mod.Name
		} else if s.Name == "Object" {
			modName = StdModName
		}

		supers[i] = Application{
			Module: modName,
			Name:   s.Name,
			Params: []SimpleType{},
		}
	}

	selfTy := Application{
		Module: self.mainModule,
		Name:   classDef.Name,
		// TODO: apply type params
		Params: []SimpleType{},
	}

	self.types.Insert("Self", selfTy)

	fields := []NamedMember{}
	methods := []NamedMember{}

	// TODO: let recursive + generalize
	for _, field := range classDef.Fields {
		switch f := field.(type) {
		case parse.ClassField:
			typ, err := self.parseType(f.Type)
			if err != nil {
				return nil, err
			}

			st, ok := typ.(SimpleType)
			if !ok {
				return nil, fmt.Errorf("type checking class member: %w", ErrIllegalTypeScheme)
			}

			fields = append(fields, NamedMember{
				Name: f.Name(),
				Member: Member{
					Type:   st,
					Access: f.Access(),
				},
			})
		case parse.ClassMethod:
			sig, ok := f.Func.Signature.Unwrap()
			if !ok {
				return nil, fmt.Errorf("typing %s.%s: %w",
					classDef.Name,
					f.Func.Name,
					ErrMethodMissingSignature,
				)
			}

			nargs := len(f.Func.Args)
			if len(sig) < nargs+1 {
				return nil, fmt.Errorf("typing %s.%s: annotated type signature too short, expected %d, got: %v",
					classDef.Name,
					f.Func.Name,
					nargs+1,
					sig,
				)
			}

			meth, err := self.parseFuncSignature(sig)
			if err != nil {
				return nil, fmt.Errorf("typing %s.%s: %w", classDef.Name, f.Func.Name, err)
			}

			methods = append(methods, NamedMember{
				Name: f.Name(),
				Member: Member{
					Type:   meth,
					Access: f.Access(),
				},
			})

		default:
			panic(fmt.Sprintf("unexpected parse.ClassMember: %#v", field))
		}
	}

	t := ObjectType{
		Module:  self.mainModule,
		Name:    classDef.Name,
		Supers:  supers,
		Fields:  fields,
		Methods: methods,
		Top:     classDef.Base,
	}

	return t, nil
}

func (self *Typer) parseFuncSignature(sig []parse.TypeRepr) (Func, error) {
	startFrom := 0
	isMethod := sig[0] == parse.SelfType{}
	argTys := make([]SimpleType, 0, len(sig))
	if isMethod {
		startFrom = 1 // skip first Self in method type
	}

	argsSig := sig[startFrom : len(sig)-1] // last element is return type
	for _, arg := range argsSig {
		param, err := self.parseType(arg)
		if err != nil {
			return Func{}, err
		}

		p := param.instantiate()
		argTys = append(argTys, p)
	}

	retHint, err := self.parseType(sig[len(sig)-1])
	if err != nil {
		return Func{}, err
	}

	return Func{
		Args:   argTys,
		Ret:    retHint.instantiate(),
		Method: isMethod,
	}, nil
}

func (self *Typer) defClassMethods(classDef *parse.ObjectTypeDef) error {
	// TODO: run the same SCC check in function dependency graph for methods

	i_meth := 0
	self.classScope = classDef.Name
	defer func() { self.classScope = "" }()
	classTy := assert.Cast[ObjectType](self.inferred[classDef.ID()], "typing class method: expected an ObjectType")

	self.vars.NewScope()
	defer self.vars.PopScope()

	self.types.NewScope()
	defer self.types.PopScope()

	self.types.Insert("Self", Application{
		Module: self.mainModule,
		Name:   classDef.Name,
		// TODO: apply variables
		Params: []SimpleType{},
	})

	for _, field := range classDef.Fields {
		// idea from ocaml, though theirs are so complicated I can't be sure I copied it
		// correctly, nor do I know if this is "safe"
		// ocaml uses limited generalization whereas I generalize the whole thing.
		// I think generalizing just the type var of the method we're check should be enough
		// TODO: pretty sure I should concretize instead?
		f, ok := field.(parse.ClassMethod)
		if !ok {
			continue
		}

		// TODO: poly type
		placeholderTy := classTy.Methods[i_meth].Type.(Func)

		// TODO: there might be a way to keep the "only top-levels can generalize" rule if
		// we make methods "polymorphic except [a, b, c]", where a,b,c are explicitly written
		// down generics at the class level
		// Q1: can't we just add a flag to Variables that are top-level (therefore should be generalized)
		// Q2: does the approach in Q1 cause problem when mixing class-bound generics and
		//     method-bound generics? if that is the case we still only need 3 "levels"
		if len(f.Func.Body) == 0 {
			// declaration, no body to type check
			return nil
		}
		fn := parse.Fn{
			Id:        f.Func.ID(),
			Signature: f.Func.Signature,
			Args:      f.Func.Args,
			Body:      f.Func.Body[len(f.Func.Body)-1],
		}

		typ, err := self.TypeTerm(&fn)
		if err != nil {
			return fmt.Errorf("typing method %s.%s: %w", classDef.Name, f.Func.Name, err)
		}

		// FIXME: this works cuz currently class and methods aren't generalized
		if err := self.symbols.constrain(placeholderTy, typ); err != nil {
			return fmt.Errorf("typing method %s.%s: %w", classDef.Name, f.Func.Name, err)
		}

		if err := self.symbols.constrain(typ, placeholderTy); err != nil {
			return fmt.Errorf("typing method %s.%s: %w", classDef.Name, f.Func.Name, err)
		}

		i_meth++
	}

	return nil
}

func (self *Typer) parseUnionDef(unionDef *parse.UnionDef) (SimpleType, error) {
	// TODO: check repeated variants?
	variants := make([]ConcreteType, len(unionDef.Variants))
	for i, v := range unionDef.Variants {
		t, err := self.parseType(v)
		if err != nil {
			return nil, err
		}

		ct, ok := t.(ConcreteType)
		if !ok {
			return nil, fmt.Errorf("%w: in definition of %s", ErrPolymorphicUnion, unionDef.Name)
		}

		variants[i] = ct
	}

	t := Union{
		Name:     unionDef.Name,
		Variants: variants,
	}
	return t, nil
}

func (self *Typer) typeFunctionCall(expr *parse.Form) (SimpleType, error) {
	assert.GreaterThan(len(expr.Children), 0, "unhandled: empty form")
	if meth, ok := expr.Children[0].(*parse.MethodAccess); ok {
		return self.typeMethodCall(meth, expr)
	}

	funcTy, err := self.TypeTerm(expr.Children[0])
	if err != nil {
		return nil, err
	}

	argTys := make([]SimpleType, len(expr.Children)-1)
	for i, arg := range expr.Children[1:] {
		ty, err := self.TypeTerm(arg)
		if err != nil {
			return nil, err
		}
		argTys[i] = ty
	}

	ret := freshVar()
	if err := self.symbols.constrain(funcTy, Func{argTys, ret, false}); err != nil {
		return nil, err
	}

	return ret, nil
}

func (self *Typer) typeRecordAccess(expr *parse.RecordAccess) (SimpleType, error) {
	if sym, ok := expr.Record.(*parse.Symbol); ok {
		if ty, ok := self.types.Get(sym.Name).Unwrap(); ok {
			return self.typeStaticMethodCall(expr, ty)
		}
	}
	recordTy, err := self.TypeTerm(expr.Record)
	if err != nil {
		return nil, err
	}

	ret := freshVar()
	err = self.symbols.constrain(recordTy, ObjectType{
		Module: "",
		Name:   "",
		// FIXME: what should Kind be?
		Supers: []Application{},
		Fields: []NamedMember{{
			Name: expr.Field,
			Member: Member{
				Type:   ret,
				Access: parse.AccessPublic, // TODO: protected/private if in class
			},
		}},
		Methods: []NamedMember{},
	})
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (self *Typer) typeMethodCall(methAccess *parse.MethodAccess, form *parse.Form) (SimpleType, error) {
	// goal:
	// 1. (class Foo {x I64, (def getx[self] self.x)})
	// 2. typing (foo#getx)
	// 3. TypeTerm(foo) -> Foo{.., getx: t1 -> t2} t1
	// 4. selectively instantiate type var of Foo::getx
	assert.GreaterThan(len(form.Children), 0, "unhandled: empty form")

	var oTy SimpleType
	switch obj := methAccess.Obj.(type) {
	case *parse.Symbol:
		if ts, ok := self.vars.Get(obj.Name).Unwrap(); ok {
			oTy = ts.instantiate()
		} else {
			return nil, fmt.Errorf("%w: %s", ErrUndefinedVariable, obj.Name)
		}
	case *parse.SelfLiteral:
		selfTy, err := self.parseType(parse.SelfType{})
		if err != nil {
			return nil, err
		}

		oTy = selfTy.instantiate()
	default:
		panic("method call on non-symbol or self not yet supported: " + methAccess.Pretty())
	}

	argTys := make([]SimpleType, 0, len(form.Children))
	for _, arg := range form.Children[1:] {
		ty, err := self.TypeTerm(arg)
		if err != nil {
			return nil, err
		}
		argTys = append(argTys, ty)
	}
	ret := freshVar()

	err := self.symbols.constrain(oTy, ObjectType{
		Module: "",
		Name:   "",
		// FIXME: what should kind be?
		Supers: []Application{},
		Fields: []NamedMember{},
		Methods: []NamedMember{{
			Name: methAccess.Method,
			Member: Member{
				Type:   Func{argTys, ret, true},
				Access: parse.AccessPublic, // TODO
			},
		}},
		Top: false,
	})
	if err != nil {
		return nil, fmt.Errorf("typing method call %s: %w", form.Pretty(), err)
	}

	// TODO: put this somewhere else
	self.inferred[methAccess.ID()] = Func{argTys, ret, true}
	self.inferred[methAccess.Obj.ID()] = oTy

	return ret, nil
}

func (self *Typer) typeStaticMethodCall(expr *parse.RecordAccess, owner TypeScheme) (SimpleType, error) {
	o := owner.instantiate()
	switch o := o.(type) {
	case ObjectType:
		for _, meth := range o.Methods {
			if meth.Name == expr.Field {
				return meth.Type, nil
			}
		}
		return nil, fmt.Errorf("%w: %s", ErrMissingStaticMethod, expr)
	default:
		panic("TODO: handle static method call on non-object type")
	}
}

func (self *Typer) parseEnumDef(enumDef *parse.EnumDef) (ConcreteType, error) {
	// TODO: check repeated variants?
	variants := map[string]opt.Option[int64]{}
	var rollingValue int64
	for _, v := range enumDef.Variants {
		if val, ok := v.Value.Unwrap(); ok {
			variants[v.Name] = opt.Some(val)
			rollingValue = val + 1
		} else {
			variants[v.Name] = opt.Some(rollingValue)
			rollingValue++
		}
	}

	t := Enum{
		Name:   enumDef.Name,
		Values: variants,
	}
	return t, nil
}

func (self *Typer) parseType(tr parse.TypeRepr) (TypeScheme, error) {
	switch t := tr.(type) {
	case parse.TypeName:
		// FIXME: when should I check for validity? e.g. undefined type, wrong type param etc.
		if t.Module == "" {
			// HACK: builtins get turned into their base type because it makes my life easier
			// (otherwise would need to export builtins to genqbe and we don't yet have mechanism
			// to separate builtins from local types :c)
			if ty, ok := builtinTypes()[t.Name]; ok {
				return ty, nil
			}
		}

		mod := self.mainModule
		if t.Module != "" {
			if m, ok := self.imports[t.Module]; ok {
				mod = m.Name
				assert.Neq(mod, "", "imported module has empty module name?")
				if _, ok := m.Types[t.Name]; !ok {
					return nil, fmt.Errorf("%w: %s.%s", ErrUndefinedTypeName, mod, t.Name)
				}
			} else {
				return nil, fmt.Errorf("%w module: %s", ErrUndefinedModule, t.Module)
			}
		}

		return Application{
			Module: mod,
			Name:   t.Name,
			Params: []SimpleType{},
		}, nil

	case parse.SelfType:
		if self.classScope == "" {
			return nil, ErrSelfTypeUnbound
		}

		if t, found := self.types.Get("Self").Unwrap(); found {
			return t, nil
		}

		return nil, fmt.Errorf("type checker bug: class scope '%s' exists but Self type not found", self.classScope)

	case parse.RecordType:
		fields, err := fun.MapIfOk(t.Fields, func(f parse.RecordTypeField) (NamedType, error) {
			t, err := self.parseType(f.Type)
			if err != nil {
				return NamedType{}, err
			}

			// TODO: should substitute type vars not instantiate
			return NamedType{Name: f.Name, Type: t.instantiate()}, nil
		})
		if err != nil {
			return nil, err
		}

		return Record{Fields: fields}, nil

	case parse.ArrayType:
		el, err := self.parseType(t.Type)
		if err != nil {
			return nil, err
		}
		e, ok := el.(SimpleType)
		if !ok {
			// cannot fail (unless nil)
			pt := el.(PolymorphicType)

			// HACK: maybe I should just make class/interfaces without type params emit
			// a simple type instead of this thing
			if typParams, ok := pt.TypeParams.Unwrap(); ok {
				if len(typParams) > 0 {
					return nil, fmt.Errorf("%w as array base type: %s", ErrIllegalPolymorphicType, el.String())
				}

				// FIXME: is instantiate ok here?
				e = pt.instantiate()
			} else {
				return nil, fmt.Errorf("%w as array base type: %s", ErrIllegalPolymorphicType, el.String())
			}
		}

		if size, ok := t.Size.Unwrap(); ok {
			if size <= 0 {
				return nil, fmt.Errorf("array size must be at least 1 (in %s)", t.String())
			}
			return ArrayType{e, uint(size)}, nil
		} else {
			return SliceType{e}, nil
		}

	case parse.TypeInstantiation:
		// FIXME: when to check validity? undefined type, wron type params etc.
		params, err := fun.MapIfOk(t.Params, func(r parse.TypeRepr) (SimpleType, error) {
			t, err := self.parseType(r)
			if err != nil {
				return nil, err
			}

			st, ok := t.(SimpleType)
			if !ok {
				return nil, fmt.Errorf("tried to pass a polymorphic type as a type parameter: %s", r)
			}
			return st, nil
		})

		if err != nil {
			return nil, err
		}

		if t.Type.Module == "" {
			// HACK: builtins get turned into their base type because it makes my life easier
			// (otherwise would need to export builtins to genqbe and we don't yet have mechanism
			// to separate builtins from local types :c)
			if ty, ok := builtinTypes()[t.Type.Name]; ok {
				if pt, ok := ty.(PolymorphicType); ok {
					return pt.concretize(params)
				}
				return nil, fmt.Errorf("%w: %s takes no parameters, %d given", ErrWrongTypeParamCount, t.Type.Name, len(params))
			}
		}

		mod := self.mainModule
		if t.Type.Module != "" {
			if m, ok := self.imports[t.Type.Module]; ok {
				mod = m.Name
			} else {
				return nil, fmt.Errorf("%w module: %s", ErrUndefinedModule, t.Type.Module)
			}
		}

		return Application{
			Module: mod,
			Name:   t.Type.Name,
			Params: params,
		}, nil

	case parse.FnType:
		args := make([]SimpleType, 0, len(t.Args))
		for _, arg := range t.Args {
			ts, err := self.parseType(arg)
			if err != nil {
				return nil, err
			}

			st, ok := ts.(SimpleType)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrIncompleteTypeInSignature, arg)
			}

			args = append(args, st)
		}

		retTs, err := self.parseType(t.Ret)
		if err != nil {
			return nil, err
		}

		ret, ok := retTs.(SimpleType)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrIncompleteTypeInSignature, t.Ret)
		}

		return Func{
			Args:   args,
			Ret:    ret,
			Method: false,
		}, nil

	default:
		panic(fmt.Sprintf("unexpected parse.TypeRepr: %#v", t))
	}
}

func (self *symbols) concretizeApplication(app Application) (SimpleType, error) {
	base, err := self.lookupType(app.Module, app.Name)
	if err != nil {
		return nil, err
	}

	switch base := base.(type) {
	case PolymorphicType:
		// FIXME: should instantiate + concretize
		ty := base.instantiate()

		return ty, nil
	case SimpleType:
		if len(app.Params) != 0 {
			panic("TODO: handle wrong parameter count?")
		}
		return base, nil
	default:
		panic(fmt.Sprintf("unexpected simplesub.TypeScheme: %#v", base))
	}
}

func (self *symbols) lookupType(module can.ModuleName, name string) (TypeScheme, error) {
	assert.Neq(module, "", "empty module name in Application should be impossible?")
	if module == self.mainModule {
		if ty, ok := self.types.Get(name).Unwrap(); ok {
			return ty, nil
		}
		return nil, fmt.Errorf("%w: %s", ErrUndefinedTypeName, name)
	}

	if mod, ok := self.moduleCache[module]; ok {
		if ty, ok := mod.Types[name]; ok {
			return ty, nil
		}
		return nil, fmt.Errorf("%w: %s.%s", ErrUndefinedTypeName, module, name)
	}
	return nil, fmt.Errorf("%w: %s", ErrUndefinedModule, module)
}

func (self *symbols) constrain(ty0 SimpleType, bound0 SimpleType) error {
	trace("constrain %v <: %v", ty0, bound0)
	indentLvl++
	defer func() { indentLvl-- }()

	// TODO: simpler-sub used type equality I think?
	if lhs, rhs, ok := matchPair[Primitive, Primitive](ty0, bound0); ok {
		if lhs.Kind == rhs.Kind {
			return nil
		}
	} else if lhs, rhs, ok := matchPair[Application, Application](ty0, bound0); ok {
		if lhs.Module == rhs.Module && lhs.Name == rhs.Name {
			return nil
		}

		return fmt.Errorf("TODO not implemented: constrain between different Application types")
	} else if lhs, _, ok := matchPair[Application, SimpleType](ty0, bound0); ok {
		lhs, err := self.concretizeApplication(lhs)
		if err != nil {
			return err
		}

		return self.constrain(lhs, bound0)
	} else if _, rhs, ok := matchPair[SimpleType, Application](ty0, bound0); ok {
		rhs, err := self.concretizeApplication(rhs)
		if err != nil {
			return err
		}

		return self.constrain(ty0, rhs)
	} else if lhs, rhs, ok := matchPair[Int, Int](ty0, bound0); ok {
		if lhs.Signed != rhs.Signed || lhs.BitSize != rhs.BitSize {
			return fmt.Errorf("%w: int conversion not implemented", ErrIncompatibleTypes)
		}
		return nil
	} else if _, _, ok := matchPair[Str, Str](ty0, bound0); ok {
		return nil
	} else if _, _, ok := matchPair[Enum, Int](ty0, bound0); ok {
		// TODO: why did I allow this??
		return nil
	} else if lhs, rhs, ok := matchPair[Ref, Ref](ty0, bound0); ok {
		return self.constrain(lhs.Content, rhs.Content)
	} else if _, rhs, ok := matchPair[Ref, Primitive](ty0, bound0); ok {
		// allow unconditional Ref <: Opaque
		if rhs.Kind == PrimitiveOpaque {
			return nil
		}
	} else if lhs, rhs, ok := matchPair[ArrayType, ArrayType](ty0, bound0); ok {
		err := self.constrain(lhs.ElType, rhs.ElType)
		if err != nil {
			return err
		}

		if lhs.Size != rhs.Size {
			return fmt.Errorf("%w %v <: %v", ErrCannotConstrain, ty0, bound0)
		}

		return nil
	} else if lhs, rhs, ok := matchPair[SliceType, SliceType](ty0, bound0); ok {
		return self.constrain(lhs.ElType, rhs.ElType)
	} else if lhs, rhs, ok := matchPair[Enum, Enum](ty0, bound0); ok {
		if lhs.Name != "" && rhs.Name != "" && lhs.Name != rhs.Name {
			return fmt.Errorf("%w: wanted %s got %s", ErrWrongEnumType, rhs.Name, lhs.Name)
		}
		for key := range rhs.Values {
			if _, ok := lhs.Values[key]; !ok {
				return fmt.Errorf("%w: %s does not have key %s", ErrEnumMissingKey, lhs, key)
			}

			// TODO: should I check the enum value as well?
		}
		return nil
	} else if lhs, rhs, ok := matchPair[Union, Union](ty0, bound0); ok {
		if lhs.Name != "" && lhs.Name == rhs.Name {
			return nil
		}

		if lhs.Name != rhs.Name {
			return fmt.Errorf("%w: wanted %s got %s", ErrWrongUnionType, rhs.Name, lhs.Name)
		}
	}

	if _, _, ok := matchPair[Bot, SimpleType](ty0, bound0); ok {
		return nil
	} else if _, _, ok := matchPair[SimpleType, Top](ty0, bound0); ok {
		return nil
	} else if ty, bound, ok := matchPair[Func, Func](ty0, bound0); ok {
		if len(ty.Args) != len(bound.Args) {
			return fmt.Errorf("%w: %v is not a subtype of %v", ErrConstraintViolated, ty, bound)
		}
		if ty.Method != bound.Method {
			return ErrMethodCallOnStaticMethod
		}

		for i, tyArg := range ty.Args {
			if err := self.constrain(bound.Args[i], tyArg); err != nil {
				return err
			}
		}
		return self.constrain(ty.Ret, bound.Ret)
	} else if ty, bound, ok := matchPair[Record, Record](ty0, bound0); ok {
		tyFields := namedTypesToMap(ty.Fields)
		for _, boundField := range bound.Fields {
			if tyField, ok := tyFields[boundField.Name]; ok {
				if err := self.constrain(tyField, boundField.Type); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("%w: missing field %s: %#v", ErrMissingField, boundField.Name, boundField.Type)
			}
		}

		return nil
	} else if ty, bound, ok := matchPair[Record, ObjectType](ty0, bound0); ok {
		// TODO: should I impl ObjectType <: Record?
		if bound.Name != "" {
			return fmt.Errorf("%w: %s cannot be used as a %s", ErrUseRecordAsNamedObjectType, ty, bound)
		}
		if len(bound.Methods) != 0 {
			return fmt.Errorf("record methods not supported: tried to use %s as a %s", ty, bound)
		}

		tyFields := namedTypesToMap(ty.Fields)
		for _, boundField := range bound.Fields {
			if tyField, ok := tyFields[boundField.Name]; ok {
				if err := self.constrain(tyField, boundField.Type); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("%w: missing field %s: %#v", ErrMissingField, boundField.Name, boundField.Type)
			}
		}

		return nil
	} else if ty, bound, ok := matchPair[ObjectType, ObjectType](ty0, bound0); ok {
		if ty.Name != "" && bound.Name != "" {
			if ty.Module == bound.Module && ty.Name == bound.Name {
				return nil
			}
			for _, sup := range ty.Supers {
				// TODO: transient super type A <: B <: C
				if sup.Module == bound.Module && sup.Name == bound.Name {
					return nil
				}
			}
			return fmt.Errorf("%w: %s.%s not a subclass of %s.%s",
				ErrInvalidClassCoercion,
				ty.Module,
				ty.Name,
				bound.Module,
				bound.Name)
		}
		tyFields := namedMembersToMap(ty.Fields)
		for _, boundField := range bound.Fields {
			if tyField, ok := tyFields[boundField.Name]; ok {
				//TODO: check visibility
				if err := self.constrain(tyField.Type, boundField.Type); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("%w %s: %v", ErrMissingField, boundField.Name, boundField.Type)
			}
		}

		tyMembers := namedMembersToMap(ty.Methods)
		for _, boundMember := range bound.Methods {
			if tyMember, ok := tyMembers[boundMember.Name]; ok {
				//TODO: check visibility
				if err := self.constrain(tyMember.Type, boundMember.Type); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("%w %s: %v\nactual type: %v",
					ErrMissingMethod, boundMember.Name, boundMember.Type,
					ty,
				)
			}
		}

		return nil
	} else if ty, bound, ok := matchPair[*Variable, *Variable](ty0, bound0); ok {
		return self.unify(ty, bound)
	} else if ty, bound, ok := matchPair[*Variable, ConcreteType](ty0, bound0); ok {
		return ty.newUpperBound(self, bound)
	} else if ty, bound, ok := matchPair[ConcreteType, *Variable](ty0, bound0); ok {
		return bound.newLowerBound(self, ty)
	} else if ty, bound, ok := matchPair[ConcreteType, Union](ty0, bound0); ok {
		for _, variant := range bound.Variants {
			if err := self.constrain(ty, variant); err == nil {
				return nil
			}
		}

		return fmt.Errorf("%w: %s is not a variant of union %s", ErrUnionUnexpectedVariant, ty.String(), bound.Name)
	} else {
		return fmt.Errorf("%w %v <: %v", ErrCannotConstrain, ty0, bound0)
	}
}

func (self *symbols) unify(lhs *Variable, rhs *Variable) error /*FIXME: idk what type*/ {
	trace("unify %s and %s", lhs.String(), rhs.String())
	indentLvl++
	defer func() { indentLvl-- }()
	rep0 := lhs.Representative()
	rep1 := rhs.Representative()

	if rep0.Uid() == rep1.Uid() {
		assert.Eq(rep0, rep1, "Same representative UID with different pointer value, uid:", rep0.Uid())
		return nil
	}

	// NOTE: these occursCheck calls (and the following ones from addXBound) are pretty
	// inefficient as they will incur repeated computation of type variables through getVars
	if err := rep0.occursCheck(rep1.lowerBound, false); err != nil {
		return err
	}

	if err := rep0.occursCheck(rep1.upperBound, true); err != nil {
		return err
	}

	if err := rep1.newLowerBound(self, rep0.lowerBound); err != nil {
		return err
	}

	if err := rep1.newUpperBound(self, rep0.upperBound); err != nil {
		return err
	}

	rep0.representative = rep1

	return nil
}

// TODO: merge with [symbols.lookupType]
func (self *symbols) lookupTypeName(tyName parse.TypeName) (TypeScheme, error) {
	if tyName.Module == "" {
		if ty, ok := self.types.Get(tyName.Name).Unwrap(); ok {
			return ty, nil
		}
		return nil, fmt.Errorf("%w: %s", ErrUndefinedTypeName, tyName.Name)
	}

	mod, ok := self.imports[tyName.Module]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUndefinedModule, tyName.Module)
	}

	if ty, ok := mod.Types[tyName.Name]; ok {
		return ty, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrUndefinedTypeName, tyName)
}

func freshVar() *Variable {
	return &Variable{uid: newId(), lowerBound: Bot{}, upperBound: Top{}}
}

// substitute all type variables in ty with fresh ones
func freshenType(ty SimpleType) SimpleType {
	freshened := map[uint]*Variable{}
	return freshenInner(freshened, ty)
}

// substitute mapped type vars with their counterpart
// does not touch type vars not present in mapping
func concretizeType(ty SimpleType, mapping map[uint]SimpleType) SimpleType {
	// TODO: I'm not sure if substituting the targeted uid is enough, what about representative?
	// is it possible to get a t1 with representative t2 but we used t1 in the mappings?
	switch t := ty.(type) {
	case *Variable:
		if match, ok := mapping[t.Uid()]; ok {
			return match
		}
		return t
	case ConcreteType:
		return substituteVarsInConcrete(t, func(child SimpleType) SimpleType {
			return concretizeType(child, mapping)
		})
	default:
		panic(fmt.Sprintf("unexpected simplesub.SimpleType: %#v", t))
	}
}

func freshenInner(freshened map[uint]*Variable, ty SimpleType) SimpleType {
	substitute := func(child SimpleType) SimpleType {
		return freshenInner(freshened, child)
	}

	switch t := ty.(type) {
	case *Variable:
		if match, ok := freshened[t.Uid()]; ok {
			return match
		} else {
			v := freshVar()
			v.lowerBound = substituteVarsInConcrete(t.LowerBound(), substitute)
			v.upperBound = substituteVarsInConcrete(t.UpperBound(), substitute)
			freshened[t.Uid()] = v
			return v
		}
	case ConcreteType:
		return substituteVarsInConcrete(t, substitute)
	default:
		panic(fmt.Sprintf("unexpected simplesub.SimpleType: %#v", t))
	}
}

func substituteVarsInConcrete(ty ConcreteType, substitute func(SimpleType) SimpleType) ConcreteType {
	switch t := ty.(type) {
	case Func:
		return Func{fun.Map(t.Args, func(arg SimpleType) SimpleType {
			return substitute(arg)
		}), substitute(t.Ret), t.Method}
	case Int:
	case Record:
		return Record{
			fun.Map(t.Fields, func(arg NamedType) NamedType {
				return NamedType{arg.Name, substitute(arg.Type)}
			}),
		}
	case ObjectType:
		return ObjectType{
			Module: t.Module,
			Name:   t.Name,
			Kind:   t.Kind,
			Supers: t.Supers,
			Fields: fun.Map(t.Fields, func(field NamedMember) NamedMember {
				return NamedMember{
					Name: field.Name,
					Member: Member{
						Type:   substitute(field.Type),
						Access: field.Access,
					},
				}
			}),
			Methods: fun.Map(t.Methods, func(meth NamedMember) NamedMember {
				return NamedMember{
					Name: meth.Name,
					Member: Member{
						Type:   substitute(meth.Type),
						Access: meth.Access,
					},
				}
			}),
			Top: t.Top,
		}
	case ArrayType:
		return ArrayType{substitute(t.ElType), t.Size}
	case SliceType:
		return SliceType{substitute(t.ElType)}
	case Ref:
		return Ref{substitute(t.Content)}
	case Application:
		return Application{
			Module: t.Module,
			Name:   t.Name,
			Params: fun.Map(t.Params, func(param SimpleType) SimpleType {
				return substitute(param)
			}),
		}
	case Primitive, Bot, Str, Top, Union, Enum: // terminals and Union, because generics are banned in Union
	default:
		panic(fmt.Sprintf("unexpected simplesub.ConcreteType: %#v", t))
	}
	return ty
}

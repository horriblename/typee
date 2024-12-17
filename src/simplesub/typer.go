package simplesub

import (
	"errors"
	"fmt"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/internal/scope"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/types"
)

type Typer struct {
	debug      bool
	classScope string
	vars       scope.ScopedMap[TypeScheme]
	types      scope.ScopedMap[TypeScheme]
}

var ErrUndefinedVariable = errors.New("undefined variable")
var ErrUndefinedTypeName = errors.New("undefined type")
var ErrSelfUnbound = errors.New("keyword self used outside of a method")
var ErrSelfTypeUnbound = errors.New("keyword Self used outside of a class definition")
var ErrWrongArgCount = errors.New("wrong argument count")
var ErrTypeMismatch = errors.New("mismatched type")
var ErrMissingField = errors.New("missing field")
var ErrMissingMethod = errors.New("missing method")
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

const scopeLevelTop int = 1

func NewTyper(debug bool) *Typer {
	vars := scope.NewScopedMap[TypeScheme]()
	addBuiltins(&vars)
	vars.NewScope()
	types := scope.NewScopedMap[TypeScheme]()
	addBuiltinTypes(&types)
	types.NewScope()
	return &Typer{
		debug: debug,
		vars:  vars,
		types: types,
	}
}

type context struct {
	inferred map[int]TypeScheme
}

func (self *Typer) TypeProgram(program []parse.Expr) ([]TypeScheme, map[int]TypeScheme, error) {
	ctx := context{map[int]TypeScheme{}}
	t, err := self.typeProgram(&ctx, program)
	return t, ctx.inferred, err
}

func (self *Typer) typeProgram(ctx *context, program []parse.Expr) ([]TypeScheme, error) {
	types := make([]TypeScheme, len(program))

	topLevels := map[string]parse.Expr{}

	recursiveness := map[string]bool{}
	groups, selfRecursives := groupRecursives(program)
	// could be optimized but eh
	for _, group := range groups {
		if len(group) == 1 {
			name := assert.Cast[string](group[0], "compiler invariant violated")
			if _, ok := selfRecursives[name]; !ok {
				recursiveness[name] = false
				continue
			}
		}

		for _, name := range group {
			name := assert.Cast[string](name, "compiler invariant violated")
			recursiveness[name] = true
		}
	}

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

		case *parse.ClassDef:
			// TODO: handle generics (parameterize class)
			types[i] = PolymorphicType{Body: freshVar()}
			self.vars.Insert(e.Name, types[i])

		case *parse.InterfaceDef:
			types[i] = PolymorphicType{Body: freshVar()}
			self.vars.Insert(e.Name, types[i])

		case *parse.UnionDef:
			types[i] = freshVar()
			self.vars.Insert(e.Name, types[i])

		case *parse.EnumDef:
			types[i] = freshVar()
			self.vars.Insert(e.Name, types[i])

		case *parse.TypeAlias:
			types[i] = freshVar()
			self.vars.Insert(e.Name, types[i])

		default:
			return nil, fmt.Errorf("%w:\n    %s", ErrInvalidTopLevel, expr.Pretty())
		}
	}

	for i, expr := range program {
		switch e := expr.(type) {
		case *parse.FuncDef:
			if len(e.Body) == 0 {
				return nil, fmt.Errorf("in function %s: %w", e.Name, ErrEmptyFuncBody)
			}
			fn := parse.Fn{
				Id:        expr.ID(),
				Signature: e.Signature,
				Args:      e.Args,
				Body:      e.Body[len(e.Body)-1],
			}
			typ, err := self.TypeTerm(ctx, &fn)
			if err != nil {
				return nil, err
			}

			fnTy, ok := types[i].(SimpleType)
			if !ok {
				pt := assert.Cast[PolymorphicType](types[i], "cannot fail")
				fnTy = pt.Body
			}
			if err := constrain(typ, fnTy); err != nil {
				return nil, err
			}

		case *parse.Set:
			typ, err := self.TypeTerm(ctx, e.Value)
			if err != nil {
				return nil, err
			}

			valTy, ok := types[i].(SimpleType)
			if !ok {
				pt := assert.Cast[PolymorphicType](types[i], "cannot fail")
				valTy = pt.Body
			}

			if err := constrain(typ, valTy); err != nil {
				return nil, err
			}

		case *parse.ClassDef:
			// TODO: idk if this is the best place to do this
			self.types.Insert(e.Name, types[i])

			t, err := self.defClass(ctx, e)
			if err != nil {
				return nil, err
			}

			pt := assert.Cast[PolymorphicType](types[i], "compiler invariant violated")
			if err := constrain(t, pt.Body); err != nil {
				return nil, err
			}

			// TODO: handle generics
			ctx.inferred[e.ID()] = t

		case *parse.UnionDef:
			self.types.Insert(e.Name, types[i])

			t, err := self.defUnion(ctx, e)
			if err != nil {
				return nil, err
			}

			st := assert.Cast[SimpleType](types[i], "compiler invariant violated")
			if err := constrain(t, st); err != nil {
				return nil, err
			}

			ctx.inferred[e.ID()] = t

		case *parse.EnumDef:
			self.types.Insert(e.Name, types[i])

			t, err := self.defEnum(e)
			if err != nil {
				return nil, err
			}

			st := assert.Cast[SimpleType](types[i], "compiler invariant violated")
			if err := constrain(t, st); err != nil {
				return nil, err
			}

			// TODO: handle generics
			ctx.inferred[e.ID()] = t

		case *parse.TypeAlias:
			typ := types[i].(SimpleType)
			self.types.Insert(e.Name, typ)

			target, err := self.parseType(e.Type)
			if err != nil {
				return nil, err
			}

			// TODO: unbound type var is not possible in types, I can "concretize"
			// the target by passing the required type var bindings instead of instantiate
			simpleTarget := target.instantiate()
			err = constrain(typ, simpleTarget)
			if err != nil {
				return nil, err
			}

			err = constrain(simpleTarget, typ)
			if err != nil {
				return nil, err
			}

		default:
			return nil, fmt.Errorf("%w:\n    %s", ErrInvalidTopLevel, expr.Pretty())
		}
	}

	return types, nil
}

func (self *Typer) typeLetRhs(ctx *context, name string, rhs parse.Expr) (PolymorphicType, error) {
	// NOTE: currently top level definitions are always recursive let,
	// and passing FuncDef as rhs is a little hack so I can write (def foo ...)
	// instead of (set foo (fn ...))
	eTy := freshVar()
	self.vars.Insert(name, eTy)
	ty, err := self.TypeTerm(ctx, rhs)
	if err != nil {
		return PolymorphicType{}, err
	}

	if err := constrain(ty, eTy); err != nil {
		return PolymorphicType{}, err
	}

	return PolymorphicType{Body: eTy}, nil
}

func (self *Typer) TypeTerm(ctx *context, term parse.Expr) (a SimpleType, _ error) {
	trace("typing: %v", term.Pretty())
	indentLvl++
	defer func() {
		indentLvl--
		trace(": %v", a)
	}()
	defer func() {
		ctx.inferred[term.ID()] = a
	}()
	switch expr := term.(type) {
	case *parse.Symbol:
		if ty, ok := self.vars.Get(expr.Name).Unwrap(); ok {
			return ty.instantiate(), nil
		} else {
			return nil, fmt.Errorf("%w: %s", ErrUndefinedVariable, expr.Name)
		}
	case *parse.SelfLiteral:
		if ty, ok := self.vars.Get("self").Unwrap(); ok {
			return ty.instantiate(), nil
		} else {
			return nil, ErrSelfUnbound
		}
	case *parse.FuncDef:
		return nil, fmt.Errorf("%w: %s", ErrDefMustBeTopLevel, expr.Name)

	case *parse.ClassDef:
		return nil, fmt.Errorf("%w: %s", ErrClassDefMustBeTopLevel, expr.Name)
	case *parse.InterfaceDef:
		return nil, fmt.Errorf("%w: %s", ErrClassDefMustBeTopLevel, expr.Name)

	case *parse.Fn:
		self.vars.NewScope()
		defer self.vars.PopScope()

		params := make([]SimpleType, len(expr.Args))
		var err error
		if sig, ok := expr.Signature.Unwrap(); ok {
			if len(sig) != len(expr.Args)+1 {
				return nil, fmt.Errorf("%w: function has %d args but type signature only takes %d", ErrBadTypeSignature, len(expr.Args), len(sig)-1)
			}

			for i, arg := range sig[:len(sig)-1] {
				param, err := self.parseType(arg)
				if err != nil {
					return nil, err
				}

				p := param.instantiate()
				params[i] = p
				self.vars.Insert(expr.Args[i], p)
			}

			bodyTy, err := self.TypeTerm(ctx, expr.Body)
			if err != nil {
				return nil, err
			}

			return Func{Args: params, Ret: bodyTy}, nil
		}

		if len(expr.Args) > 0 && expr.Args[0] == "self" {
			ty, err := self.parseType(parse.SelfType{})
			if err != nil {
				return nil, err
			}

			self.vars.Insert("self", ty)
		}

		for i, arg := range expr.Args {
			param := freshVar()
			params[i] = param
			self.vars.Insert(arg, param)
		}
		bodyTy, err := self.TypeTerm(ctx, expr.Body)
		if err != nil {
			return nil, err
		}

		return Func{Args: params, Ret: bodyTy}, nil

	case *parse.Form:
		assert.GreaterThan(len(expr.Children), 0, "unhandled: empty form")

		funcTy, err := self.TypeTerm(ctx, expr.Children[0])
		if err != nil {
			return nil, err
		}

		argTys := make([]SimpleType, len(expr.Children)-1)
		for i, arg := range expr.Children[1:] {
			ty, err := self.TypeTerm(ctx, arg)
			if err != nil {
				return nil, err
			}
			argTys[i] = ty
		}

		ret := freshVar()
		if err := constrain(funcTy, Func{argTys, ret}); err != nil {
			return nil, err
		}

		return ret, nil

	case *parse.New:
		if class, ok := self.types.Get(expr.Class).Unwrap(); ok {
			return Func{[]SimpleType{}, class.instantiate()}, nil
		}
		return nil, fmt.Errorf("%w: %s", ErrUndefinedTypeName, expr.Class)

	case *parse.BoolLiteral:
		return Bool{}, nil
	case *parse.IntLiteral:
		return Int{}, nil
	case *parse.StrLiteral:
		return Str{}, nil
	case *parse.ArrayLiteral:
		elTyps := make([]SimpleType, len(expr.Elements))
		for i, el := range expr.Elements {
			elTy, err := self.TypeTerm(ctx, el)
			if err != nil {
				return nil, err
			}

			elTyps[i] = elTy
		}

		ty := freshVar()
		for _, elTy := range elTyps {
			if err := constrain(elTy, ty); err != nil {
				return nil, fmt.Errorf("array element has incompatible type with other elements before it: %w", err)
			}
		}

		return ArrayType{ty, uint(len(expr.Elements))}, nil

	case *parse.Record:
		fields := make([]NamedType, len(expr.Fields))
		for i, field := range expr.Fields {
			fieldTy, err := self.TypeTerm(ctx, field.Value)
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
		// TODO: allow non-variable as record
		recordTy, err := self.TypeTerm(ctx, expr.Record)
		if err != nil {
			return nil, err
		}

		ret := freshVar()
		err = constrain(recordTy, ObjectType{
			Name:   "",
			Supers: []ObjectType{},
			Fields: []NamedMember{{
				Name: expr.Field,
				Member: Member{
					Type:   ret,
					Access: types.AccessPublic, // TODO: protected/private if in class
				},
			}},
			Methods: []NamedMember{},
		})
		if err != nil {
			return nil, err
		}

		return ret, nil

	case *parse.MethodAccess:
		objTy, err := self.TypeTerm(ctx, expr.Var)
		if err != nil {
			return nil, err
		}

		ret := freshVar()
		err = constrain(objTy, ObjectType{
			Name:   "",
			Supers: []ObjectType{},
			Fields: []NamedMember{},
			Methods: []NamedMember{{
				Name: expr.Method,
				Member: Member{
					Type:   ret,
					Access: types.AccessPublic, // TODO
				},
			}},
		})
		if err != nil {
			return nil, err
		}

		return ret, nil

	case *parse.EnumAccess:
		// FIXME: uh, actually type check this pls
		ts, ok := self.types.Get(expr.Enum).Unwrap()
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUndefinedTypeName, expr.Enum)
		}

		// FIXME: shouldn't this be a PolymorphicType?
		enum, ok := ts.(Enum)
		if !ok {
			return nil, fmt.Errorf("%w: %s is a %v", ErrNotEnum, expr.Enum, ts)
		}

		return enum, nil

	case *parse.IfExpr:
		condTy, err := self.TypeTerm(ctx, expr.Condition)
		if err != nil {
			return nil, err
		}

		if err := constrain(condTy, Bool{}); err != nil {
			return nil, err
		}

		retTy := freshVar()
		thenTy, err := self.TypeTerm(ctx, expr.Consequence)
		if err != nil {
			return nil, err
		}

		elseTy, err := self.TypeTerm(ctx, expr.Alternative)
		if err != nil {
			return nil, err
		}

		if err := constrain(thenTy, retTy); err != nil {
			return nil, err
		}

		if err := constrain(elseTy, retTy); err != nil {
			return nil, err
		}
		return retTy, nil

	case *parse.LetExpr:
		if expr.Recursive {
			if self.vars.ScopeLevel() <= scopeLevelTop {
				// TODO
				panic("TODO")
			}
		} else {
			argValues := fun.Map(expr.Assignments, func(ass parse.Assignment) parse.Expr {
				return ass.Value
			})
			argNames := fun.Map(expr.Assignments, func(ass parse.Assignment) string {
				return ass.Var
			})
			return self.TypeTerm(ctx, &parse.Form{
				Children: append([]parse.Expr{
					&parse.Fn{
						Id:   expr.ID(),
						Args: argNames,
						Body: expr.Body,
					},
				}, argValues...),
			})
		}
	case *parse.CaseExpr:
	case *parse.Set:
		varTy, ok := self.vars.Get(expr.Name).Unwrap()
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUndefinedVariable, expr.Name)
		}

		val, err := self.TypeTerm(ctx, expr.Value)
		if err != nil {
			return nil, err
		}

		varSTy, ok := varTy.(SimpleType)
		if !ok {
			panic("TODO: call set on PolymorphicType")
		}

		err = constrain(val, varSTy)
		if err != nil {
			return nil, err
		}

		return Record{[]NamedType{}}, err

	case *parse.VarDef:
		// TODO: should var be a let rec?
		val, err := self.TypeTerm(ctx, expr.Value)
		if err != nil {
			return nil, err
		}

		self.vars.Insert(expr.Name, val)
		return Record{[]NamedType{}}, err

	case *parse.TaggedExpr:
	case *parse.ExternCall:
		for _, arg := range expr.Args {
			self.TypeTerm(ctx, arg)
		}

		return freshVar(), nil

	default:
		panic(fmt.Sprintf("unexpected parse.Expr: %#v", expr))
	}
	panic(fmt.Sprintf("unhandled: TypeTerm(%s)", term.Pretty()))
}

func (self *Typer) defClass(ctx *context, classDef *parse.ClassDef) (SimpleType, error) {
	self.classScope = classDef.Name
	defer func() { self.classScope = "" }()
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
			// TODO: there might be a way to keep the "only top-levels can generalize" rule if
			// we make methods "polymorphic except [a, b, c]", where a,b,c are explicitly written
			// down generics at the class level
			if len(f.Func.Body) == 0 {
				return nil, fmt.Errorf("in function %s: %w", f.Func.Name, ErrEmptyFuncBody)
			}
			fn := parse.Fn{
				Id:        f.Func.ID(),
				Signature: f.Func.Signature,
				Args:      f.Func.Args,
				Body:      f.Func.Body[len(f.Func.Body)-1],
			}
			typ, err := self.typeLetRhs(ctx, f.Name(), &fn)
			if err != nil {
				return nil, err
			}

			methods = append(methods, NamedMember{
				Name: f.Name(),
				Member: Member{
					Type:   typ.Body, // FIXME
					Access: f.Access(),
				},
			})

		default:
			panic(fmt.Sprintf("unexpected parse.ClassMember: %#v", field))
		}
	}

	supers := make([]ObjectType, len(classDef.Supers))
	for i, s := range classDef.Supers {
		sup, ok := self.types.Get(s).Unwrap()
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUndefinedTypeName, s)
		}

		sc, ok := sup.(ObjectType)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrIllegalSuperType, s)
		}

		supers[i] = sc
	}

	t := ObjectType{
		Name:    classDef.Name,
		Supers:  supers,
		Fields:  fields,
		Methods: methods,
	}
	self.types.Insert(classDef.Name, t)
	return t, nil
}

func (self *Typer) defUnion(ctx *context, unionDef *parse.UnionDef) (SimpleType, error) {
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
	self.types.Insert(unionDef.Name, t)
	return t, nil
}

func (self *Typer) defEnum(enumDef *parse.EnumDef) (SimpleType, error) {
	// TODO: check repeated variants?
	variants := map[string]int64{}
	var rollingValue int64
	for _, v := range enumDef.Variants {
		if val, ok := v.Value.Unwrap(); ok {
			variants[v.Name] = val
			rollingValue = val + 1
		} else {
			variants[v.Name] = rollingValue
			rollingValue++
		}
	}

	t := Enum{
		Name:   enumDef.Name,
		Values: variants,
	}
	self.types.Insert(enumDef.Name, t)
	return t, nil
}

func (self *Typer) parseType(tr parse.TypeRepr) (TypeScheme, error) {
	switch t := tr.(type) {
	case parse.TypeName:
		if typ, ok := self.types.Get(t.Name).Unwrap(); ok {
			return typ, nil
		}
		return nil, fmt.Errorf("%w: %s", ErrUndefinedTypeName, t.Name)
	case parse.SelfType:
		if self.classScope == "" {
			return nil, ErrSelfTypeUnbound
		}

		if t, found := self.types.Get(self.classScope).Unwrap(); found {
			return t, nil
		}

		panic(fmt.Sprintf("type checker bug: class scope '%s' exists but corresponding type not found", self.classScope))

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
			return nil, fmt.Errorf("%w as array base type: %s", ErrIllegalPolymorphicType, t.String())
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
		base, err := self.parseType(t.Type)
		if err != nil {
			return nil, err
		}

		pbase, ok := base.(PolymorphicType)
		if !ok {
			return nil, fmt.Errorf("%w: type %s in %s", ErrUnparameterizedTypePassedParams, base, t)
		}

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

		return pbase.concretize(params)

	default:
		panic(fmt.Sprintf("unexpected parse.TypeRepr: %#v", t))
	}
}

func constrain(ty0 SimpleType, bound0 SimpleType) error {
	trace("constrain %v <: %v", ty0, bound0)
	indentLvl++
	defer func() { indentLvl-- }()
	// TODO: simpler-sub used type equality I think?
	if _, _, ok := matchPair[Bool, Bool](ty0, bound0); ok {
		return nil
	} else if _, _, ok := matchPair[Int, Int](ty0, bound0); ok {
		return nil
	} else if _, _, ok := matchPair[Str, Str](ty0, bound0); ok {
		return nil
	} else if _, _, ok := matchPair[Enum, Int](ty0, bound0); ok {
		return nil
	} else if lhs, rhs, ok := matchPair[ArrayType, ArrayType](ty0, bound0); ok {
		err := constrain(lhs.ElType, rhs.ElType)
		if err != nil {
			return err
		}

		if lhs.Size != rhs.Size {
			return fmt.Errorf("%w %v <: %v", ErrCannotConstrain, ty0, bound0)
		}

		return nil
	} else if lhs, rhs, ok := matchPair[SliceType, SliceType](ty0, bound0); ok {
		return constrain(lhs.ElType, rhs.ElType)
	} else if lhs, rhs, ok := matchPair[Enum, Enum](ty0, bound0); ok {
		if lhs.Name != rhs.Name {
			return fmt.Errorf("%w: wanted %s got %s", ErrWrongEnumType, rhs.Name, lhs.Name)
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
			return fmt.Errorf("%w: %#v is not a subtype of %#v", ErrConstraintViolated, ty, bound)
		}

		for i, tyArg := range ty.Args {
			if err := constrain(bound.Args[i], tyArg); err != nil {
				return err
			}
		}
		return constrain(ty.Ret, bound.Ret)
	} else if ty, bound, ok := matchPair[Record, Record](ty0, bound0); ok {
		tyFields := namedTypesToMap(ty.Fields)
		for _, boundField := range bound.Fields {
			if tyField, ok := tyFields[boundField.Name]; ok {
				if err := constrain(tyField, boundField.Type); err != nil {
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
				if err := constrain(tyField, boundField.Type); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("%w: missing field %s: %#v", ErrMissingField, boundField.Name, boundField.Type)
			}
		}

		return nil
	} else if ty, bound, ok := matchPair[ObjectType, ObjectType](ty0, bound0); ok {
		tyMembers := namedMembersToMap(ty.Methods)
		for _, boundMember := range bound.Methods {
			if tyMember, ok := tyMembers[boundMember.Name]; ok {
				//TODO: check visibility
				if err := constrain(tyMember.Type, boundMember.Type); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("%w %s: %v", ErrMissingMethod, boundMember.Name, boundMember.Type)
			}
		}

		return nil
	} else if ty, bound, ok := matchPair[*Variable, *Variable](ty0, bound0); ok {
		return unify(ty, bound)
	} else if ty, bound, ok := matchPair[*Variable, ConcreteType](ty0, bound0); ok {
		return ty.newUpperBound(bound)
	} else if ty, bound, ok := matchPair[ConcreteType, *Variable](ty0, bound0); ok {
		return bound.newLowerBound(ty)
	} else if ty, bound, ok := matchPair[ConcreteType, Union](ty0, bound0); ok {
		for _, variant := range bound.Variants {
			if concreteEq(ty, variant) {
				return nil
			}
		}

		return fmt.Errorf("%w: %s is not a variant of union %s", ErrUnionUnexpectedVariant, ty.String(), bound.Name)
	} else {
		return fmt.Errorf("%w %v <: %v", ErrCannotConstrain, ty0, bound0)
	}
}

func unify(lhs *Variable, rhs *Variable) error /*FIXME: idk what type*/ {
	trace("unify %s and %s", lhs.String(), rhs.String())
	indentLvl++
	defer func() { indentLvl-- }()
	rep0 := lhs.Representative()
	rep1 := rhs.Representative()

	// FIXME: deep equality or pointer eq?
	if rep0 != rep1 {
		// NOTE: these occursCheck calls (and the following ones from addXBound) are pretty
		// inefficient as they will incur repeated computation of type variables through getVars
		if err := lhs.occursCheck(rep1.lowerBound, false); err != nil {
			return err
		}

		if err := lhs.occursCheck(rep1.upperBound, true); err != nil {
			return err
		}

		if err := rep1.newLowerBound(rep0.lowerBound); err != nil {
			return err
		}

		if err := rep1.newUpperBound(rep0.upperBound); err != nil {
			return err
		}

		rep0.representative = rep1
	}
	return nil
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
		return Func{
			fun.Map(t.Args, func(arg SimpleType) SimpleType {
				return substitute(arg)
			}),
			substitute(t.Ret),
		}
	case Int:
	case Record:
		return Record{
			fun.Map(t.Fields, func(arg NamedType) NamedType {
				return NamedType{arg.Name, substitute(arg.Type)}
			}),
		}
	case ObjectType:
		fields := fun.Map(t.Fields, func(arg NamedMember) NamedMember {
			return NamedMember{arg.Name, Member{substitute(arg.Type), arg.Access}}
		})
		methods := fun.Map(t.Methods, func(arg NamedMember) NamedMember {
			return NamedMember{arg.Name, Member{substitute(arg.Type), arg.Access}}
		})

		return ObjectType{
			Name:    t.Name,
			Supers:  []ObjectType{},
			Fields:  fields,
			Methods: methods,
		}
	case Bool, Bot, Str, Top, Union, Enum: // terminals and Union, because generics are banned in Union
	default:
		panic(fmt.Sprintf("unexpected simplesub.ConcreteType: %#v", t))
	}
	return ty
}

func assertCast[O any](x any, msg any) O {
	if o, ok := x.(O); ok {
		return o
	} else {
		panic(fmt.Sprintf("cast from %v to %T failed. %v", x, o, msg))
	}
}

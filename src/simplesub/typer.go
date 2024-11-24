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

func (self *Typer) TypeProgram(program []parse.Expr) ([]PolymorphicType, map[int]TypeScheme, error) {
	ctx := context{map[int]TypeScheme{}}
	t, err := self.typeProgram(&ctx, program)
	return t, ctx.inferred, err
}

func (self *Typer) typeProgram(ctx *context, program []parse.Expr) ([]PolymorphicType, error) {
	types := make([]PolymorphicType, len(program))
	for i, expr := range program {
		switch e := expr.(type) {
		case *parse.FuncDef:
			types[i] = PolymorphicType{freshVar()}
			self.vars.Insert(e.Name, types[i])

		case *parse.Set:
			types[i] = PolymorphicType{freshVar()}
			self.vars.Insert(e.Name, types[i])

		case *parse.ClassDef:
			types[i] = PolymorphicType{freshVar()}
			self.vars.Insert(e.Name, types[i])
		case *parse.UnionDef:
			types[i] = PolymorphicType{freshVar()}
			self.vars.Insert(e.Name, types[i])

		case *parse.EnumDef:
			types[i] = PolymorphicType{freshVar()}
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

			if err := constrain(typ, types[i].Body); err != nil {
				return nil, err
			}

		case *parse.Set:
			typ, err := self.TypeTerm(ctx, e.Value)
			if err != nil {
				return nil, err
			}

			if err := constrain(typ, types[i].Body); err != nil {
				return nil, err
			}

		case *parse.ClassDef:
			// TODO: idk if this is the best place to do this
			self.types.Insert(e.Name, types[i])

			t, err := self.defClass(ctx, e)
			if err != nil {
				return nil, err
			}

			if err := constrain(t, types[i].Body); err != nil {
				return nil, err
			}

			ctx.inferred[e.ID()] = PolymorphicType{t}

		case *parse.UnionDef:
			self.types.Insert(e.Name, types[i])

			t, err := self.defUnion(ctx, e)
			if err != nil {
				return nil, err
			}

			if err := constrain(t, types[i].Body); err != nil {
				return nil, err
			}

			ctx.inferred[e.ID()] = PolymorphicType{t}

		case *parse.EnumDef:
			self.types.Insert(e.Name, types[i])

			t, err := self.defEnum(e)
			if err != nil {
				return nil, err
			}

			if err := constrain(t, types[i].Body); err != nil {
				return nil, err
			}

			ctx.inferred[e.ID()] = PolymorphicType{t}

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

	return PolymorphicType{eTy}, nil
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

				p, ok := param.(SimpleType)
				if !ok {
					return nil, fmt.Errorf("%w: signature contains global quantifier: %v", ErrBadTypeSignature, arg)
				}

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
		if err := lhs.occursCheck(rep1.LowerBound(), false); err != nil {
			return err
		}

		if err := lhs.occursCheck(rep1.UpperBound(), true); err != nil {
			return err
		}

		if err := rep1.newLowerBound(rep0.LowerBound()); err != nil {
			return err
		}

		if err := rep1.newUpperBound(rep0.UpperBound()); err != nil {
			return err
		}

		rep0.representative = rep1
	}
	return nil
}

func freshVar() *Variable {
	return &Variable{uid: newId(), lowerBound: Bot{}, upperBound: Top{}}
}

func freshenType(ty SimpleType) SimpleType {
	freshened := map[*Variable]*Variable{}
	return freshenInner(freshened, ty)
}

func freshenInner(freshened map[*Variable]*Variable, ty SimpleType) SimpleType {
	switch t := ty.(type) {
	case *Variable:
		if match, ok := freshened[t]; ok {
			return match
		} else {
			v := freshVar()
			v.lowerBound = freshenConcrete(freshened, t.LowerBound())
			v.upperBound = freshenConcrete(freshened, t.UpperBound())
			freshened[t] = v
			return v
		}
	case ConcreteType:
		return freshenConcrete(freshened, t)
	default:
		panic(fmt.Sprintf("unexpected simplesub.SimpleType: %#v", t))
	}

}

func freshenConcrete(freshened map[*Variable]*Variable, ty ConcreteType) ConcreteType {
	switch t := ty.(type) {
	case Func:
		return Func{
			fun.Map(t.Args, func(arg SimpleType) SimpleType {
				return freshenInner(freshened, arg)
			}),
			freshenInner(freshened, t.Ret),
		}
	case Int:
	case Record:
		return Record{
			fun.Map(t.Fields, func(arg NamedType) NamedType {
				return NamedType{arg.Name, freshenInner(freshened, arg.Type)}
			}),
		}
	case ObjectType:
		fields := fun.Map(t.Fields, func(arg NamedMember) NamedMember {
			return NamedMember{arg.Name, Member{freshenInner(freshened, arg.Type), arg.Access}}
		})
		methods := fun.Map(t.Methods, func(arg NamedMember) NamedMember {
			return NamedMember{arg.Name, Member{freshenInner(freshened, arg.Type), arg.Access}}
		})

		return ObjectType{
			Name:    t.Name,
			Supers:  []ObjectType{},
			Fields:  fields,
			Methods: methods,
		}
	case Bool, Bot, Str, Top, Union: // terminals and Union, because generics are banned in Union
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

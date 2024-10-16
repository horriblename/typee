package biunify

import (
	"errors"
	"fmt"
	"slices"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/internal/scope"
	"github.com/horriblename/typee/src/parse"
)

type Typer struct {
	debug bool
	vars  scope.ScopedMap[TypeScheme]
}

var ErrUndefinedVariable = errors.New("undefined variable")
var ErrWrongArgCount = errors.New("wrong argument count")
var ErrTypeMismatch = errors.New("mismatched type")
var ErrMissingField = errors.New("missing field")

const scopeLevelTop int = 1

func NewTyper(debug bool) *Typer {
	return &Typer{
		debug: debug,
		vars:  scope.NewScopedMap[TypeScheme](),
	}
}

func (self *Typer) TypeTerm(term parse.Expr) (SimpleType, error) {
	switch expr := term.(type) {
	case *parse.Symbol:
		if ty, ok := self.vars.Get(expr.Name).Unwrap(); ok {
			return ty.instantiate(), nil
		} else {
			return nil, fmt.Errorf("%w: %s", ErrUndefinedVariable, expr.Name)
		}
	case *parse.FuncDef:
		self.vars.NewScope()
		defer self.vars.PopScope()

		params := make([]SimpleType, len(expr.Args))
		for i, arg := range expr.Args {
			param := freshVar()
			params[i] = param
			self.vars.Insert(arg, param)
		}
		bodyTy, err := self.TypeTerm(expr.Body[len(expr.Body)-1])
		if err != nil {
			return nil, err
		}
		return Func{Args: params, Ret: bodyTy}, err

	case *parse.Form:
		assert.GreaterThan(len(expr.Children), 0, "unhandled: empty form")

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
		if err := constrain(funcTy, Func{argTys, ret}); err != nil {
			return nil, err
		}

		return ret, nil
	case *parse.BoolLiteral:
		return Bool{}, nil

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
		// TODO: allow non-variable as record
		recordTy, err := self.TypeTerm(&parse.Symbol{Name: expr.Record})
		if err != nil {
			return nil, err
		}

		ret := freshVar()
		constrain(recordTy, Record{[]NamedType{{expr.Field, ret}}})
		return ret, nil

	case *parse.IfExpr:
		condTy, err := self.TypeTerm(expr.Condition)
		if err != nil {
			return nil, err
		}

		if err := constrain(condTy, Bool{}); err != nil {
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
			return self.TypeTerm(&parse.Form{
				Children: append([]parse.Expr{
					&parse.Fn{
						Args: argNames,
						Body: expr.Body,
					},
				}, argValues...),
			})
		}
	case *parse.CaseExpr:
	case *parse.Fn:
	case *parse.IntLiteral:
	case *parse.Set:
	case *parse.StrLiteral:
	case *parse.TaggedExpr:
	default:
		panic(fmt.Sprintf("unexpected parse.Expr: %#v", expr))
	}
	panic(fmt.Sprintf("unhandled: TypeTerm(%s)", term.Pretty()))
}

func constrain(ty SimpleType, bound SimpleType) error {
	// TODO: simpler-sub used pointer equality (I think) to skip constrain here
	if _, ok := bound.(Top); ok {
		return nil
	}

	switch ty := ty.(type) {
	case Bot:
	case Bool:
		if _, ok := bound.(Bool); ok {
			return nil
		} else {
			panic(fmt.Sprintf("unhandled: %#v :< %#v", ty, bound))
		}
	case Func:
		boundTy, ok := bound.(Func)
		if !ok {
			break
		}

		if len(ty.Args) != len(boundTy.Args) {
			return ErrWrongArgCount
		}

		for i, tyArg := range ty.Args {
			if err := constrain(tyArg, boundTy.Args[i]); err != nil {
				return err
			}
		}
		if err := constrain(ty.Ret, boundTy.Ret); err != nil {
			return err
		}
	case Record:
		boundTy, ok := bound.(Record)
		if !ok {
			break
		}

		for _, tyField := range ty.Fields {
			// TODO: having separate types for polar types will eliminate this search...
			i := slices.IndexFunc(boundTy.Fields,
				func(field NamedType) bool { return field.Name == tyField.Name })
			if i == -1 {
				return fmt.Errorf("%w: %s", ErrMissingField, tyField.Name)
			}

			if err := constrain(tyField.Type, boundTy.Fields[i].Type); err != nil {
				return err
			}
		}

	case Variable:
		switch bound := bound.(type) {
		case Variable:
			unify(ty, bound)
		case ConcreteType:
			ty.newUpperBound(bound)
		}
	default:
	}

	ty0, ok0 := ty.(ConcreteType)
	ty1, ok1 := bound.(Variable)
	if ok0 && ok1 {
		ty1.newLowerBound(ty0)
		return nil
	}

	return fmt.Errorf("cannot constrain: %#v <: %#v", ty, bound)
}

func unify(lhs Variable, rhs Variable) SimpleType /*FIXME: idk what type*/ {
	panic("unimpl")
}

func freshVar() Variable {
	return Variable{
		lowerBound: Bot{},
		upperBound: Top{},
	}
}

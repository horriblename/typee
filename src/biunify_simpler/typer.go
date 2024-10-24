package biunify

import (
	"errors"
	"fmt"

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
var ErrConstraintViolated = errors.New("constraint violated")
var ErrCannotConstrain = errors.New("cannot constrain")

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
		if err := constrain(recordTy, Record{[]NamedType{{expr.Field, ret}}}); err != nil {
			return nil, err
		}

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

func constrain(ty0 SimpleType, bound0 SimpleType) error {
	// TODO: simpler-sub used type equality I think?
	if ty0 == bound0 {
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
			if err := constrain(tyArg, bound.Args[i]); err != nil {
				return err
			}

		}
		return constrain(ty.Ret, bound.Ret)
	} else if ty, bound, ok := matchPair[Record, Record](ty0, bound0); ok {
		tyFields := namedTypesToMap(ty.Fields)
		for _, boundField := range bound.Fields {
			if tyField, ok := tyFields[boundField.Name]; ok {
				if err := constrain(boundField.Type, tyField); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("%w: missing field %s: %#v", ErrMissingField, boundField.Name, boundField.Type)
			}
		}

		return nil
	} else if ty, bound, ok := matchPair[Variable, Variable](ty0, bound0); ok {
		return unify(ty, bound)
	} else if ty, bound, ok := matchPair[Variable, ConcreteType](ty0, bound0); ok {
		return ty.newUpperBound(bound)
	} else if ty, bound, ok := matchPair[ConcreteType, Variable](ty0, bound0); ok {
		return bound.newLowerBound(ty)
	} else {
		return fmt.Errorf("%w %#v <: %#v", ErrCannotConstrain, ty0, bound0)
	}
}

func unify(lhs Variable, rhs Variable) error /*FIXME: idk what type*/ {
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

func freshVar() Variable {
	return Variable{
		lowerBound: Bot{},
		upperBound: Top{},
	}
}

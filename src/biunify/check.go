package biunify

import (
	"errors"
	"fmt"
	"strings"

	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/parse"
)

var ErrUndefinedVariable = errors.New("undefined variable")
var ErrDuplicateField = errors.New("duplicated field")
var ErrDuplicateCaseBranch = errors.New("duplicate case branch")
var ErrEmptyFunctionDef = errors.New("missing function body")
var ErrEmptyForm = errors.New("empty form")

type FieldType struct {
	Name string
	Type Value
}

var tempIndent = "  "
var tempIndentLvl = 0

// CheckExpr is the main API entrypoint for the type checker
func CheckExpr(engine TypeChecker, bindings Bindings, expr parse.Expr) (val Value, err error) {
	fmt.Printf("%s\x1b[33m%s\x1b[0m\n", strings.Repeat(tempIndent, tempIndentLvl), expr.Pretty())
	tempIndentLvl++
	defer func() {
		tempIndentLvl--
		fmt.Printf("%svalue: #%d%#v\n", strings.Repeat(tempIndent, tempIndentLvl), val.ID, engine.Head(val.ID))
	}()
	switch expr := expr.(type) {
	case *parse.BoolLiteral:
		return engine.Bool(), nil
	case *parse.IntLiteral:
		return engine.Int(), nil
	case *parse.StrLiteral:
		return engine.Str(), nil
	case *parse.Symbol:
		val, found := bindings.get(expr.Name).Unwrap()
		if !found {
			return Value{}, fmt.Errorf("looking for %v: %w", val, ErrUndefinedVariable)
		}
		return val, nil

	case *parse.Record:
		fieldNames := map[string]struct{}{}
		fieldTypePairs := []NamedValue{}
		for _, field := range expr.Fields {
			if _, found := fieldNames[field.Name]; found {
				return Value{}, fmt.Errorf("checking record field %s: %w", field.Name, ErrDuplicateField)
			}

			t, err := CheckExpr(engine, bindings, field.Value)
			if err != nil {
				return Value{}, err
			}

			fieldTypePairs = append(fieldTypePairs, NamedValue{Name: field.Name, Value: t})
		}

		return engine.Obj(fieldTypePairs), nil
	case *parse.TaggedExpr:
		valType, err := CheckExpr(engine, bindings, expr.Body)
		if err != nil {
			return Value{}, err
		}

		return engine.Tagged(NamedValue{expr.Tag, valType}), nil
	case *parse.IfExpr:
		condTy, err := CheckExpr(engine, bindings, expr.Condition)
		bound := engine.BoolUse()
		if err != nil {
			return Value{}, err
		}
		err = engine.Flow(condTy, bound)
		if err != nil {
			return Value{}, err
		}

		thenTy, err := CheckExpr(engine, bindings, expr.Consequence)
		if err != nil {
			return Value{}, err
		}
		elseTy, err := CheckExpr(engine, bindings, expr.Alternative)
		if err != nil {
			return Value{}, err
		}

		merged, mergedBound := engine.Var()
		err = engine.Flow(thenTy, mergedBound)
		if err != nil {
			return Value{}, err
		}

		err = engine.Flow(elseTy, mergedBound)
		if err != nil {
			return Value{}, err
		}

		return merged, nil

	case *parse.RecordAccess:
		// FIXME: non-variable record access
		lhsExpr := expr.Record
		lhsTy, err := CheckExpr(engine, bindings, lhsExpr)
		if err != nil {
			return Value{}, err
		}

		fieldTy, fieldBound := engine.Var()
		bound := engine.ObjUse(NamedUse{Name: expr.Field, Use: fieldBound})
		err = engine.Flow(lhsTy, bound)
		if err != nil {
			return Value{}, err
		}

		return fieldTy, nil

	case *parse.CaseExpr:
		matchTy, err := CheckExpr(engine, bindings, expr.Match)
		if err != nil {
			return Value{}, err
		}
		resultTy, resultBound := engine.Var()

		caseNames := map[string]struct{}{}
		caseTypePairs := []NamedUse{}
		for _, branch := range expr.Branches {
			if exists := mapInsert(caseNames, branch.Pattern.Tag, struct{}{}); exists {
				return Value{}, fmt.Errorf("%w: %s", ErrDuplicateCaseBranch, branch.Pattern.Tag)
			}

			wrappedTy, wrappedBound := engine.Var()
			caseTypePairs = append(caseTypePairs, NamedUse{branch.Pattern.Tag, wrappedBound})

			bindings.NewScope()
			bindings.insert(branch.Pattern.Pattern, wrappedTy)
			rhsTy, err := CheckExpr(engine, bindings, branch.Body)
			bindings.PopScope()
			if err != nil {
				return Value{}, err
			}

			err = engine.Flow(rhsTy, resultBound)
			if err != nil {
				return Value{}, err
			}
		}

		bound := engine.TaggedUse(caseTypePairs)
		err = engine.Flow(matchTy, bound)
		if err != nil {
			return Value{}, err
		}

		return resultTy, nil
	case *parse.FuncDef:
		bindings.NewScope()
		defer bindings.PopScope()
		argBounds := fun.Map(expr.Args, func(arg string) Use {
			argTy, argBound := engine.Var()
			bindings.insert(arg, argTy)
			return argBound
		})

		if len(expr.Body) == 0 {
			return Value{}, ErrEmptyFunctionDef
		}

		retTy, err := CheckExpr(engine, bindings, expr.Body[len(expr.Body)-1])
		if err != nil {
			return Value{}, err
		}

		return engine.Func(argBounds, retTy), nil
	case *parse.Form:
		if len(expr.Children) == 0 {
			return Value{}, ErrEmptyForm
		}

		funcTy, err := CheckExpr(engine, bindings, expr.Children[0])
		if err != nil {
			return Value{}, err
		}

		argTys := make([]Value, len(expr.Children)-1)
		for i, arg := range expr.Children[1:] {
			argTys[i], err = CheckExpr(engine, bindings, arg)
			if err != nil {
				return Value{}, err
			}
		}

		retTy, retBound := engine.Var()
		bound := engine.FuncUse(argTys, retBound)
		err = engine.Flow(funcTy, bound)
		if err != nil {
			return Value{}, err
		}

		return retTy, nil

	case *parse.LetExpr:
		if expr.Recursive {
			return checkLetRecExpr(engine, bindings, expr)
		}
		varTys := make([]Value, len(expr.Assignments))
		for i, ass := range expr.Assignments {
			varTy, err := CheckExpr(engine, bindings, ass.Value)
			if err != nil {
				return Value{}, err
			}
			varTys[i] = varTy
		}

		bindings.NewScope()
		defer bindings.PopScope()
		for i, varTy := range varTys {
			bindings.insert(expr.Assignments[i].Var, varTy)
		}

		return CheckExpr(engine, bindings, expr.Body)
	}
	panic("unreachable")
}

func checkLetRecExpr(engine TypeChecker, bindings Bindings, expr *parse.LetExpr) (Value, error) {
	if !expr.Recursive {
		panic("tried to call checkLetRecExpr on non-recursive let expr")
	}

	tempBounds := make([]Use, len(expr.Assignments))
	for i, ass := range expr.Assignments {
		// first create temp type variables
		tempTy, tempBound := engine.Var()
		bindings.insert(ass.Var, tempTy)
		tempBounds[i] = tempBound
	}

	for i, bound := range tempBounds {
		varTy, err := CheckExpr(engine, bindings, expr.Assignments[i].Value)
		if err != nil {
			return Value{}, err
		}

		// flow the "real" type to the temp bound
		err = engine.Flow(varTy, bound)
		if err != nil {
			return Value{}, err
		}
	}

	return CheckExpr(engine, bindings, expr.Body)
}

func mapInsert[K comparable, V any](m map[K]V, k K, v V) (overwritten bool) {
	_, overwritten = m[k]
	m[k] = v
	return overwritten
}

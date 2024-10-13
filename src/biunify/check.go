package biunify

import (
	"errors"
	"fmt"

	"github.com/horriblename/typee/src/parse"
)

var ErrUndefinedVariable = errors.New("undefined variable")
var ErrDuplicateField = errors.New("duplicated field")
var ErrDuplicateCaseBranch = errors.New("duplicate case branch")

type FieldType struct {
	Name string
	Type Value
}

func CheckExpr(engine TypeChecker, bindings Bindings, expr parse.Expr) (Value, error) {
	switch expr := expr.(type) {
	case *parse.IntLiteral:
		engine.Int()
	case *parse.BoolLiteral:
		engine.Bool()
	case *parse.Symbol:
		val, found := bindings.get(expr.Name).Unwrap()
		if !found {
			return nil, fmt.Errorf("looking for %s: %w", val, ErrUndefinedVariable)
		}
		return val, nil

	case *parse.Record:
		fieldNames := map[string]struct{}{}
		fieldTypePairs := []NamedValue{}
		for _, field := range expr.Fields {
			if _, found := fieldNames[field.Name]; found {
				return nil, fmt.Errorf("checking record field %s: %w", field.Name, ErrDuplicateField)
			}

			t, err := CheckExpr(engine, bindings, field.Value)
			if err != nil {
				return nil, err
			}

			fieldTypePairs = append(fieldTypePairs, NamedValue{Name: field.Name, Value: t})
		}

		return engine.Obj(fieldTypePairs), nil
	case *parse.TaggedExpr:
		valType, err := CheckExpr(engine, bindings, expr.Body)
		if err != nil {
			return nil, err
		}

		return engine.Tagged(expr.Tag, valType), nil
	case *parse.IfExpr:
		condTy, err := CheckExpr(engine, bindings, expr.Condition)
		bound := engine.BoolUse()
		if err != nil {
			return nil, err
		}
		engine.Flow(condTy, bound)

		thenTy, err := CheckExpr(engine, bindings, expr.Consequence)
		if err != nil {
			return nil, err
		}
		elseTy, err := CheckExpr(engine, bindings, expr.Alternative)
		if err != nil {
			return nil, err
		}

		merged, mergedBound := engine.Var()
		engine.Flow(thenTy, mergedBound)
		engine.Flow(elseTy, mergedBound)
		return merged, nil

	case *parse.RecordAccess:
		// FIXME: non-variable record access
		lhsExpr := parse.Symbol{Name: expr.Record}
		lhsTy, err := CheckExpr(engine, bindings, &lhsExpr)
		if err != nil {
			return nil, err
		}

		fieldTy, fieldBound := engine.Var()
		bound := engine.ObjUse(NamedUse{Name: expr.Field, Use: fieldBound})
		err = engine.Flow(lhsTy, bound)
		if err != nil {
			return nil, err
		}

		return fieldTy, nil

	case *parse.CaseExpr:
		matchTy, err := CheckExpr(engine, bindings, expr.Match)
		if err != nil {
			return nil, err
		}
		resultTy, resultBound := engine.Var()

		caseNames := map[string]struct{}{}
		caseTypePairs := []NamedUse{}
		for _, branch := range expr.Branches {
			if exists := mapInsert(caseNames, branch.Pattern.Tag, struct{}{}); exists {
				return nil, fmt.Errorf("%w: %s", ErrDuplicateCaseBranch, branch.Pattern.Tag)
			}

			wrappedTy, wrappedBound := engine.Var()
			caseTypePairs = append(caseTypePairs, NamedUse{branch.Pattern.Tag, wrappedBound})

			bindings.NewScope()
			bindings.insert(branch.Pattern.Pattern, wrappedTy)
			rhsTy, err := CheckExpr(engine, bindings, branch.Body)
			if err != nil {
				return nil, err
			}

			err = engine.Flow(rhsTy, resultBound)
			if err != nil {
				return nil, err
			}
		}

		bound := engine.TaggedUse(caseTypePairs)
		err = engine.Flow(matchTy, bound)
		if err != nil {
			return nil, err
		}

		return resultTy, nil
	}
	panic("unimpl")
}

func mapInsert[K comparable, V any](m map[K]V, k K, v V) (overwritten bool) {
	_, overwritten = m[k]
	m[k] = v
	return overwritten
}

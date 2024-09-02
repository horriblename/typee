package solve

import (
	"errors"
	"fmt"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/types"
)

var (
	ErrUnifyFailed = errors.New("could not unify")
)

type Subst struct {
	Old types.Type
	New types.Type
}

func Check(topLevels []parse.Expr) (SymbolTable, error) {
	ss := ScopeStack{}
	ss.AddScope()
	for _, expr := range topLevels {
		typ, err := infer(&ss, expr)
		if err != nil {
			return nil, fmt.Errorf("infering %s: %w", expr.Pretty(), err)
		}

		switch e := expr.(type) {
		case *parse.FuncDef:
			err := ss.DefSymbol(e.Name, typ)
			if err != nil {
				return nil, fmt.Errorf("assigning inferred type to function definition: %w", err)
			}
		default:
			panic("non-function definitions not supported")
		}
	}

	assert.Eq(len(ss.stack), 1, "scope stack size > 1 after type checking top levels")

	return ss.Pop(), nil
}

func Infer(expr parse.Expr) (types.Type, error) {
	return infer(&ScopeStack{}, expr)
}

func infer(ss *ScopeStack, expr parse.Expr) (types.Type, error) {
	typ, cons, err := initConstraints(ss, expr)
	if err != nil {
		return nil, fmt.Errorf("generate constraints: %w", err)
	}

	subs, err := unify(cons)
	if err != nil {
		return nil, fmt.Errorf("unify: %w", err)
	}

	substituteAllToType(&typ, subs)
	return typ, nil
}

func unify(cs []Constraint) ([]Subst, error) {
	_, subs, err := unifyInner(cs, []Subst{})
	return subs, err
}

func unifyInner(cs []Constraint, subs []Subst) ([]Constraint, []Subst, error) {
	if len(cs) == 0 {
		return cs, subs, nil
	}

	c := cs[0]
	if c.lhs.Simple() && c.lhs.Eq(c.rhs) {
		return unifyInner(cs[1:], subs)
	}

	if lhs, ok := c.lhs.(*types.Generic); ok {
		hasLhs := false
		visitor := func(typ *types.Type) Stop {
			if lhs.Eq(*typ) {
				hasLhs = true
				return Break
			}
			return Continue
		}

		walkTypeUntil(&c.rhs, visitor)
		if !hasLhs {
			sub := Subst{Old: lhs, New: c.rhs}
			substituteConstraintSet(cs[1:], sub)
			subs = append(subs, sub)
		}
		return unifyInner(cs[1:], subs)
	}

	if rhs, ok := c.rhs.(*types.Generic); ok {
		hasRhs := false
		visitor := func(typ *types.Type) Stop {
			if rhs.Eq(*typ) {
				hasRhs = true
				return Break
			}
			return Continue
		}

		walkTypeUntil(&c.lhs, visitor)
		if !hasRhs {
			sub := Subst{Old: rhs, New: c.lhs}
			substituteConstraintSet(cs[1:], sub)
			subs = append(subs, sub)
		}
		return unifyInner(cs[1:], subs)
	}

	// break one constraint down into two smaller constraints and add those constraints
	// back in to be further unified
	if lhs, ok := c.lhs.(*types.Func); ok {
		if rhs, ok := c.rhs.(*types.Func); ok {
			if len(lhs.Args) != len(rhs.Args) {
				return nil, nil, ErrWrongArgCount
			}

			// FIXME: allocates per prepend
			csNew := []Constraint{{lhs.Ret, rhs.Ret}}
			for i, larg := range lhs.Args {
				csNew = append(csNew, Constraint{larg, rhs.Args[i]})
			}
			cs = append(csNew, cs[1:]...)
			return unifyInner(cs, subs)
		}
	}

	return nil, nil, fmt.Errorf("constraint %s = %s, %w", c.lhs, c.rhs, ErrUnifyFailed)
}

func substituteConstraintSet(cons []Constraint, sub Subst) {
	for i := range cons {
		substituteType(&cons[i].lhs, sub)
		substituteType(&cons[i].rhs, sub)
	}
}

func substituteAllToType(typ *types.Type, subs []Subst) {
	for _, s := range subs {
		substituteType(typ, s)
	}
}

func substituteType(typ *types.Type, sub Subst) {
	visitor := func(node *types.Type) Stop {
		if (*node).Eq(sub.Old) {
			*node = sub.New
		}
		return Continue
	}

	walkTypeUntil(typ, visitor)
}

type Stop bool

const (
	Break    Stop = true
	Continue Stop = false
)

// Utils

func (s Subst) String() string {
	return fmt.Sprintf("{%v / %v}", s.New, s.Old)
}

func walkTypeUntil(typ *types.Type, visitor func(node *types.Type) Stop) {
	switch t := (*typ).(type) {
	case *types.Bool, *types.Int, *types.String, *types.Generic:
		if visitor(typ) {
			return
		}
	case *types.TypeScheme:
		if visitor(typ) {
			return
		}

		walkTypeUntil(&t.Body, visitor)
	case *types.Func:
		if visitor(typ) {
			return
		}

		for i := range t.Args {
			walkTypeUntil(&t.Args[i], visitor)
		}

		walkTypeUntil(&t.Ret, visitor)
	case *types.Record:
		if visitor(typ) {
			return
		}

		for _, field := range t.Fields {
			walkTypeUntil(&field, visitor)
		}
	}
}

func walkAST(node parse.Expr, walker func(node parse.Expr)) {
	switch n := node.(type) {
	case *parse.Form:
		walker(node)
		for _, child := range n.Children {
			walkAST(child, walker)
		}
	case *parse.Symbol, *parse.StrLiteral, *parse.IntLiteral, *parse.BoolLiteral:
		walker(node)
	case *parse.FuncDef:
		walker(node)
		for _, child := range n.Body {
			walkAST(child, walker)
		}
	case *parse.Fn:
		walker(node)
		walkAST(n.Body, walker)
	case *parse.Set:
		walker(node)
		walkAST(n.Value, walker)
	case *parse.IfExpr:
		walker(node)
		walkAST(n.Condition, walker)
		walkAST(n.Consequence, walker)
		walkAST(n.Alternative, walker)
		// case *parse.
	case *parse.LetExpr:
		walker(node)
		for _, ass := range n.Assignments {
			walkAST(ass.Value, walker)
		}
		walkAST(n.Body, walker)
	case *parse.Record:
		walker(node)
		for _, field := range n.Fields {
			walker(field.Value)
		}
	}

	panic("unknown ast node")
}

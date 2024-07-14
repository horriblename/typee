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
		assert.True(!hasLhs, "recursive types not supported")
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
		assert.True(!hasRhs, "recursive types not supported")
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
			assert.Eq(len(lhs.Args), 1, "only single arguments supported, got: ", lhs)
			assert.Eq(len(rhs.Args), 1, "only single arguments supported, got: ", rhs)

			// FIXME: allocates per prepend
			cs = append([]Constraint{{lhs.Args[0], rhs.Args[0]}}, cs[1:]...)
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
	case *types.Func:
		if visitor(typ) {
			return
		}

		for i := range t.Args {
			walkTypeUntil(&t.Args[i], visitor)
		}

		walkTypeUntil(&t.Ret, visitor)
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
	case *parse.Set:
		walker(node)
		walkAST(n.Value, walker)
	case *parse.IfExpr:
		walker(node)
		walkAST(n.Condition, walker)
		walkAST(n.Consequence, walker)
		walkAST(n.Alternative, walker)
	}

	panic("unknown ast node")
}

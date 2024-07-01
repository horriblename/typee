// package solve provides the type inference algorithm.
//
// # Algortihm overview
//
// The Algorithm is split into 2 parts:
// 1. generate constraints (for each expression / AST node)
// 2. unify constraint sets
//
// # Constraint generation rules, notation
//
// NOTE: we're using OCaml-like type notation in this section, `'t` is a
// generic type and `t` is a concrete type. Later on in our actual program we'll
// use PascalCase for concrete types and snakeCase for type variables. I might
// mix them up at some point >:3.
//
//	env |- e : t -| C
//
// This reads as: in environment `env`, expression `e` is inferred to have type
// `t` and generates constraint set `C`.
//   - A constraint is an equation of the form `t1 = t2` for any types `t1` and
//     `t2`
//   - The `e : t` in the middle is roughly what you see in the toplevel: you
//     enter an expression, and it tells you the type, but around that is an
//     environment and constraint set `env |- ... -| C` that is invisible to
//     you. So, the turnstiles around the outside show the parts of type
//     inference that the toplevel does not
//
// # Notation Example
//
//	env |- i : Int -| {}
//
// The above means: an integer constant (aka literal), is known to have type
// Int, and there are no constraints generated
//
// # Variable constraints
//
// Inferring the *type of a name* requires looking it up in the environment:
//
//	env |- n : env(n) -| {}
//
// # No constraints are generated
//
// # If expression constraints
//
// Here's the rule for `if` expressions:
//
//	env |- (if [e1] e2 e3) : 't -| C1, C2, C3, t1 = bool, 't = t2, 't = t3
//		if fresh 't
//		and env |- e1 : t1 -| C1
//		and env |- e2 : t2 -| C2
//		and env |- e3 : t3 -| C3
//
// To infer the type of an `if`, we infer the types	`t1`, `t2`, and `t3` of
// each of its subexpressions, along with any constraints on them. We know that
// the type of the condition must be `bool`. So we generate a constraint
// `t1 = bool`
//
// Furthermore, we know that both branches must have the same type - though, we
// don't really know what that type might be. So, we invent a *fresh* type
// variable `'t` to stand for that type. A type variable is fresh if it has
// never been used elsewhere during type inference. So, picking a fresh type
// variable just means picking a new name that can't conflict with other names.
// We return `'t` as the type of the `if`, and we record two constraints `'t =
// t2` and `'t = t3' to say that both branches must have the type.
//
// We therefore need to add type variables to the syntax of types:
//
//	t ::= 'x | int | bool | t1 -> t2
package solve

import (
	"github.com/horriblename/typee/src/parse"
)

type TypeVar struct {
	id         TypeID
	identifier bool
}

type Subst struct {
	Target TypeVar
	Repl   TypeVar
}

type Constraint struct {
	lhs ExprID
	rhs TypeVar
}

type ExprID = int

func initConstraints(node parse.Expr) ([]Constraint, error) {
	constraints := []Constraint{}
	tt := NewTypeTable()
	err := genConstraints(tt, &constraints, node)
	if err != nil {
		return nil, err
	}

	return constraints, nil
}

func genConstraints(tt TypeTable, constraints *[]Constraint, node parse.Expr) error {
	switch node.(type) {
	case *parse.IntLiteral, *parse.BoolLiteral, *parse.StrLiteral:
		// no constraints generated for "constant" types
	case *parse.IfExpr:
	default:
		panic("unhandled node type in genConstraints")
	}

	return nil
}

var gId = 0

func newId() int {
	id := gId
	gId++
	return id
}

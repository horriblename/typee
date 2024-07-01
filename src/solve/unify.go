// package solve provides the type inference algorithm.
//
// # Algortihm overview
//
// The Algorithm is split into 2 parts:
// 1. generate constraints (for each expression / AST node)
// 2. unify/solve constraint sets
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
// To infer the type of an `if`, we infer the types `t1`, `t2`, and `t3` of
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
//
//	{} |- (if [true] 1 0) : 't -| bool = bool, 't = int
//		{} |- true : bool -| {}
//		{} |- 1 : int -| {}
//		{} |- 0 : int -| {}
//
// # Anonymous functions
//
// Since there is no type annotation on x, its type must be inferred:
//
//	env |- fun x -> e : 't1 -> t2 -| C
//		if fresh 't1
//		and env, x : 't1 |- e : t2 -| C
//
// We introduce a fresh type variable `'t1` to stand for the type of `x`, and
// infer the type of body `e` under the environment in which `x : 't1`. Whenever
// `x` is used in `e`, that can cause constraints to be generated involving
// `'t1`. Those constraints will become part of `C`
//
// Here's a function where we can immediately see that `x : bool`, but let's
// work through the inference:
//
//	{} |- fun x -> if x then 1 else 0 : 't1 -> 't -| 't1 = bool, 't = int
//		{}, x : 't1 |- if x then 1 else 0 : 't -| 't1 = bool, 't = int
//			{}, x : 't1 |- x : 't1 -| {}
//			{}, x : 't1 |- 1 : int -| {}
//			{}, x : 't1 |- 0 : int -| {}
//
// The inferred type of the function is `'t1 -> 't`, with constraints `'t1 =
// bool` and `'t = int`. Simplfying that, the function's type is `bool -> int`
//
// # Function Application
//
// The type of the entire application must be inferred, because we don't yet
// know anything about the types of either subexpression:
//
//	env |- e1 e2 : 't -| C1, C2, t1 = t2 -> 't
//		if fresh 't
//		and env |- e1 : t1 -| C1
//		and env |- e2 : t2 -| C2
//
// We introduce a fresh type variable `'t` for the type of the application
// expression. We use inference to determine the types of the subexpressions and
// any constraints they happen to generate. We add one new constraint, `t1 = t2
// -> 't`, which expresses that the type of the left-hand side `e1` must be a
// function that takes in an argument of type t2 and returns a value of type
// `'t`.
//
// Let `I` be the *initial environment* that binds the boolean operators. Let's
// infer the type of a partial application of `( + )`:
//
//	I |- ( + ) 1 : 't -| int -> int -> int = int -> 't
//		I |- ( + ) : int -> int -> int -| {}
//		I |- 1 : int -| {}
//
// From the resulting constraint, we see that
//
//	int -> int -> int
//	=
//	int -> 't
//
// stripping the `int ->` off the left-hand side of each of those function
// types, we are left with
//
//	int -> int = 't
//
// Hence, the type of `( + ) 1` is `int -> int`.
//
// # Solving Constraints
//
// What does it mean to solve a set of constraints? Since constraints are
// equations on types, it's much like solving a system of equations in algebra.
// We want to solve for the values of the variables appearing in those
// equations. By substituting those values for the variables, we should get
// equations that are identical on both sides. For example, in algebra we might
// have:
//
//	5x + 2y = 9
//	x - y = -1
//
// Solving that system, we'd get that x=1 and y=2. If we substitute 1 for x and
// 2 for y, we get:
//
//	5(1) + 2(2) = 9
//	1 - 2 = -1
//
// which reduces to
//
//	9 = 9
//	-1 = -1
//
// In programming languages terminology (though perhaps not high-school
// algebra), we say that the substitutions `{1 / x}` and `{2 / y}` together
// *unify* that set of equations, because they make each equation "unit" such
// that its left side is identical to its right side
//
// Solving systems of equations on types is similar. Just as we found numbers to
// substitute for variables above, we now want to find types to substitute for
// type variables, and thereby unify the set of equations.
//
// Much like the substitutions we defined before for the substitution model of
// evaluation, we'll write `{t / 'x}` for the *type substitution* that maps type
// variable `'x` to type `t`. For example, `t1 {t2/'x}` means type t1 with t2
// substituted for `'x`.
//
// We can define substitution on types as follows:
//
//	int {t / 'x} = int
//	bool {t / 'x} = bool
//	'x {t / 'x} = t
//	'y {t / 'x} = 'y
//	(t1 -> t2) {t / 'x} = (t1 {t / 'x}) -> (t2 {t / 'x})
//
// Given two substitutions `S1` and `S2`, we write `S1; S2` to mean the
// substitution that is their *sequential composition*, which is defined as
// follows:
//
//	t (S1; S2) = (t S1) S2
//
// The order matters. For example, `'x ({('y -> 'y) / 'x}; {bool / 'y})` is
// `bool -> bool`, not `'y -> 'y`. We can build up bigger and bigger
// substitutions this way.
//
// A substitution `S` can be applied to a constraint `t = t'`. The result `(t =
// t') S` is defined to be `t S = t' S`. So we just apply the substitution on
// both sides of the constraint.
//
// Finally, a substitution can be applied to a set `C` of constraints; the
// result of `C S` is the result of applying `S` to each of the individual
// constraints in `C`.
//
// A substitution *unifies* a constraint `t_1 = t_2` if `t_1 S` results in the
// same type as `t_2 S`. For example, substitution `S = {int -> int / 'y; {int /
// 'x}}` unifies constraint `'x -> ('x -> int) = int -> 'y`, because
//
//	('x -> ('x -> int)) S
//	=
//	int -> (int -> int)
//
// and
//
//	(int -> 'y) S
//	=
//	int -> (int -> int)
//
// A substitution S unifies a set C of constraints if S unifies every constraint
// in C.
//
// At last, we can precisely say what it means to solve a set of constraints: we
// must find a substitution that unifies the set. That is, we need to find a
// sequence of maps from type variables to types, such that the sequence causes
// each equation in the constraint set to "unite", meaning that its left-hand
// side and right-hand side become the same.
//
// To find a substitution that unifies constraint set C, we use an algorithm
// `unify`, which is defined as follows:
//
// 1. if C is the empty set, then `unify(C)` is the empty substitution.
// 2. if C contains at least on constraint `t1 = t2` and possibly some other
// constraints `C'`, then `unify(C)` is defined as follows:
//   - If `t1` and `t2` are both the same simple type - i.e., both the same type
//     variable `'x`, or both `int` or both `bool` - then return `unify(C')`.
//     *In this case, the constraint contained no useful information, so we're
//     tossing it out and continuing.*
//   - If `t1` is a type variable `'x` and `'x` does not occur in t2, then let
//     `S = {t2 / 'x}`, and return `S; unify(C' S)`. *In this case, we are
//     eliminating the variable `'x` from the system of equations, much like
//     Gaussian elimination in solving algebraic equations.
//   - If t2 is a type variable `'x` and `'x` does not occur in t1, then let `S
//     = {t1 / 'x}`, and return `S; unify(C' S)`. *This is an elimination like
//     the previous case.*
//   - If `t1 = i1 -> o1` and `t2 = i2 -> o2`, where i1, i2, o1, and o2 are
//     types, then `unify(i1 = i2,)`
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

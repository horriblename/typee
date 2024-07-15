package parse

import (
	"errors"
	"fmt"

	"github.com/horriblename/typee/src/combinator"
	"github.com/horriblename/typee/src/lex"
)

var ErrParse = errors.New("parse error")
var LexError = errors.New("error in lex")

type internalError struct{ error }

var gIdCounter = 1

func newId() int {
	id := gIdCounter
	gIdCounter++
	return id
}

func Program(in []lex.Token) ([]lex.Token, []Expr, error) {
	return combinator.Many(expr)(in)
}

func ParseString(source string) ([]Expr, error) {
	tokens, err := lex.LexString(source)
	if err != nil {
		fmt.Printf("tokens: %v", tokens)
		return nil, errors.Join(LexError, err)
	}

	_, prog, err := Program(tokens)
	return prog, err
}

func expr(in []lex.Token) ([]lex.Token, Expr, error) {
	return combinator.Any(
		formLike,
		symbol,
		strLiteral,
		intLiteral,
		kwTrue,
		kwFalse,
	)(in)
}

func strLiteral(in []lex.Token) ([]lex.Token, Expr, error) {
	if len(in) == 0 {
		return nil, nil, errAt(in)
	}

	if lit, ok := in[0].(*lex.StrLiteral); ok {
		return in[1:], &StrLiteral{id: newId(), Content: lit.Content}, nil
	}

	return nil, nil, errAt(in)
}

func intLiteral(in []lex.Token) ([]lex.Token, Expr, error) {
	if len(in) == 0 {
		return nil, nil, errAt(in)
	}

	if lit, ok := in[0].(*lex.IntLiteral); ok {
		return in[1:], &IntLiteral{id: newId(), Number: lit.Number}, nil
	}

	return nil, nil, errAt(in)
}

func formLike(in []lex.Token) ([]lex.Token, Expr, error) {
	if len(in) == 0 {
		return nil, nil, errAt(in)
	}
	if _, ok := in[0].(*lex.LParen); !ok {
		return nil, nil, errAt(in)
	}

	switch at(in, 1).(type) {
	case *lex.Def:
		return defForm(in)

	case *lex.Set:
		return setForm(in)

	case *lex.If:
		return ifExpr(in)

	case *lex.Let:
		return letExpr(in)

	case *lex.Fn:
		return fnExpr(in)

	case nil:
		return nil, nil, errAt(in)

	default:
		return form(in)
	}
}

func form(in []lex.Token) (rest []lex.Token, exp Expr, err error) {
	rest, out, err := combinator.Surround(
		lparen,
		combinator.Many(expr),
		rparen,
	)(in)

	return rest, &Form{id: newId(), Children: out}, err
}

func defForm(in []lex.Token) (_ []lex.Token, _ Expr, err error) {
	defer func() { err = handleCheck(recover()) }()

	in, _, err = lparen(in)
	check(err)

	in, _, err = kwDef(in)
	check(err)

	in, name, err := symbolName(in)
	check(err)

	// optional (T1 T2 ...)
	in, sig, err := combinator.Maybe(combinator.Surround(
		lparen,
		combinator.Many(symbolName),
		rparen,
	))(in)
	check(err)

	// [x y z ...]
	in, args, err := combinator.Surround(lbracket, combinator.Many0(symbolName), rbracket)(in)
	check(err)

	in, body, err := combinator.Many(expr)(in)
	check(err)

	in, _, err = rparen(in)
	check(err)

	if sig, exist := sig.Unwrap(); exist && len(sig) != len(args)+1 {
		return nil, nil, errors.New("function signature does not match arguments")
	}

	def := FuncDef{
		id:        newId(),
		Name:      name,
		Signature: sig,
		Args:      args,
		Body:      body,
	}

	return in, &def, nil
}

func setForm(in []lex.Token) (_ []lex.Token, _ Expr, err error) {
	defer func() { err = handleCheck(recover()) }()

	in, _, err = lparen(in)
	check(err)

	in, _, err = kwSet(in)
	check(err)

	in, lval, err := symbolName(in)
	check(err)

	in, rval, err := expr(in)
	check(err)

	setExpr := &Set{
		id:    newId(),
		Name:  lval,
		Value: rval,
	}

	return in, setExpr, nil
}

func ifExpr(in []lex.Token) (_ []lex.Token, _ Expr, err error) {
	defer func() { err = handleCheck(recover()) }()

	in, _, err = lparen(in)
	check(err)

	in, _, err = kwIf(in)
	check(err)

	in, cond, err := combinator.Surround(
		lbracket,
		expr,
		rbracket,
	)(in)
	check(err)

	in, cons, err := expr(in)
	check(err)

	in, alt, err := expr(in)
	check(err)

	in, _, err = rparen(in)
	check(err)

	return in, &IfExpr{
		id:          newId(),
		Condition:   cond,
		Consequence: cons,
		Alternative: alt,
	}, nil
}

func letExpr(in []lex.Token) (_ []lex.Token, _ Expr, err error) {
	defer func() { err = handleCheck(recover()) }()

	in, _, err = lparen(in)
	check(err)

	in, _, err = kwLet(in)
	check(err)

	in, ass, err := combinator.Surround(
		lbracket,
		combinator.Many0(assignment),
		rbracket,
	)(in)
	check(err)

	in, body, err := expr(in)
	check(err)

	in, _, err = rparen(in)
	check(err)

	let := &LetExpr{
		id:          newId(),
		Assignments: ass,
		Body:        body,
	}
	return in, let, nil
}

func assignment(in []lex.Token) (_ []lex.Token, _ Assignment, err error) {
	defer func() { err = handleCheck(recover()) }()

	in, name, err := symbolName(in)
	check(err)

	in, body, err := expr(in)
	check(err)

	return in, Assignment{Var: name, Value: body}, nil
}

func fnExpr(in []lex.Token) (_ []lex.Token, _ Expr, err error) {
	defer func() { err = handleCheck(recover()) }()

	in, _, err = lparen(in)
	check(err)

	in, _, err = kwFn(in)
	check(err)

	in, arg, err := combinator.Surround(
		lbracket,
		symbolName,
		rbracket,
	)(in)
	check(err)

	in, body, err := expr(in)
	check(err)

	in, _, err = rparen(in)
	check(err)

	return in, &Fn{Arg: arg, Body: body}, nil
}

func symbol(in []lex.Token) ([]lex.Token, Expr, error) {
	if len(in) == 0 {
		return nil, nil, errAt(in)
	}

	if sym, ok := in[0].(*lex.Symbol); ok {
		return in[1:], &Symbol{id: newId(), Name: sym.Name}, nil
	} else {
		return nil, nil, errAt(in)
	}
}

func symbolName(in []lex.Token) ([]lex.Token, string, error) {
	if len(in) == 0 {
		return nil, "", errAt(in)
	}

	if sym, ok := in[0].(*lex.Symbol); ok {
		return in[1:], sym.Name, nil
	} else {
		return nil, "", errAt(in)
	}
}

func lparen(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.LParen])(in)
}
func rparen(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.RParen])(in)
}
func lbracket(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.LBracket])(in)
}
func rbracket(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.RBracket])(in)
}
func kwDef(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Def])(in)
}
func kwSet(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Set])(in)
}
func kwIf(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.If])(in)
}
func kwLet(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Let])(in)
}
func kwFn(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Fn])(in)
}
func kwTrue(in []lex.Token) ([]lex.Token, Expr, error) {
	rest, _, err := wrappedResult(matchOne[*lex.TrueLiteral])(in)
	if err != nil {
		return nil, nil, err
	}

	return rest, &BoolLiteral{id: newId(), Value: true}, nil
}
func kwFalse(in []lex.Token) ([]lex.Token, Expr, error) {
	rest, _, err := wrappedResult(matchOne[*lex.FalseLiteral])(in)
	if err != nil {
		return nil, nil, err
	}

	return rest, &BoolLiteral{id: newId(), Value: false}, nil

}

func matchOne[T lex.Token](in []lex.Token) ([]lex.Token, struct{}, error) {
	if len(in) == 0 {
		return nil, struct{}{}, errAt(in)
	}

	if _, ok := in[0].(T); ok {
		return in[1:], struct{}{}, nil
	} else {
		return nil, struct{}{}, errAt(in)
	}
}

// returns in[idx] or nil if out of range
func at(in []lex.Token, idx int) lex.Token {
	if idx < 0 || idx >= len(in) {
		return nil
	}

	return in[idx]
}

func check(err error) {
	if err != nil {
		panic(&internalError{err})
	}
}

func handleCheck(err any) error {
	if err != nil {
		if err, ok := err.(*internalError); ok {
			return err.error
		} else {
			panic(err)
		}
	}

	return nil
}

// Error Handling

func errAt(in []lex.Token) error {
	return wrapIfErr(in, ErrParse)
}

func wrapIfErr(in []lex.Token, err error) error {
	if err == nil {
		return nil
	}

	if len(in) == 0 {
		return fmt.Errorf("at the end: %w", err)
	} else if len(in) <= 10 {
		return fmt.Errorf("at '%s': %w", in, err)
	}

	return fmt.Errorf("at '%s...': %w", in[:10], err)
}

func wrappedResult[I ~[]lex.Token, O any](parser combinator.Parser[I, O]) combinator.Parser[I, O] {
	return func(in I) (I, O, error) {
		rest, out, err := parser(in)
		return rest, out, wrapIfErr(in, err)
	}
}

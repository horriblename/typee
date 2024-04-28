package parse

import (
	"errors"
	"fmt"

	"github.com/horriblename/typee/src/combinator"
	"github.com/horriblename/typee/src/lex"
)

var ErrParse = errors.New("parse error")
var LexError = errors.New("error in lex")

func program(in []lex.Token) ([]lex.Token, []Expr, error) {
	return combinator.Many(expr)(in)
}

func parseString(source string) ([]Expr, error) {
	tokens, err := lex.LexString(source)
	if err != nil {
		fmt.Printf("tokens: %v", tokens)
		return nil, errors.Join(LexError, err)
	}

	_, prog, err := program(tokens)
	return prog, err
}

func expr(in []lex.Token) ([]lex.Token, Expr, error) {
	return combinator.Any(
		form,
		symbol,
	)(in)
}

func form(in []lex.Token) ([]lex.Token, Expr, error) {
	rest, out, err := combinator.Surround(
		lparen,
		combinator.Many(expr),
		rparen,
	)(in)

	return rest, &Form{children: out}, err
}

func symbol(in []lex.Token) ([]lex.Token, Expr, error) {
	if len(in) == 0 {
		return nil, nil, ErrParse
	}

	if sym, ok := in[0].(*lex.Symbol); ok {
		return in[1:], &Symbol{Name: sym.Name}, nil
	} else {
		return nil, nil, ErrParse
	}
}

func lparen(in []lex.Token) ([]lex.Token, struct{}, error) { return matchOne[*lex.LParen](in) }
func rparen(in []lex.Token) ([]lex.Token, struct{}, error) { return matchOne[*lex.RParen](in) }

func matchOne[T lex.Token](in []lex.Token) ([]lex.Token, struct{}, error) {
	if len(in) == 0 {
		return nil, struct{}{}, ErrParse
	}

	if _, ok := in[0].(T); ok {
		return in[1:], struct{}{}, nil
	} else {
		return nil, struct{}{}, ErrParse
	}
}

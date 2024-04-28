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
		formLike,
		symbol,
	)(in)
}

func formLike(in []lex.Token) ([]lex.Token, Expr, error) {
	if len(in) == 0 {
		return nil, nil, ErrParse
	}
	if _, ok := in[0].(*lex.LParen); !ok {
		return nil, nil, ErrParse
	}

	switch at(in, 1).(type) {
	case *lex.Def:
		return defForm(in)

	case *lex.Set:
		panic("unimpl")

	case nil:
		return nil, nil, ErrParse
	}

	rest, out, err := combinator.Surround(
		lparen,
		combinator.Many(expr),
		rparen,
	)(in)

	return rest, &Form{children: out}, err
}

type internalError struct{ error }

// func (self *internalError) Error() string  { return self.error.Error() }

func defForm(in []lex.Token) (rest []lex.Token, exp Expr, err error) {
	defer func() {
		if e := recover(); e != nil {
			if e, ok := e.(*internalError); ok {
				err = e.error
			} else {
				panic(err)
			}
		}
	}()

	in, _, err = lparen(in)
	check(err)

	in, _, err = kwDef(in)
	check(err)

	in, name, err := symbolName(in)
	check(err)

	in, args, err := combinator.Surround(
		lparen,
		combinator.Many(symbolName),
		rparen,
	)(in)

	in, body, err := combinator.Many(expr)(in)
	check(err)

	def := FuncDef{
		Name: name,
		Args: args,
		Body: body,
	}

	return in, &def, nil
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

func symbolName(in []lex.Token) ([]lex.Token, string, error) {
	if len(in) == 0 {
		return nil, "", ErrParse
	}

	if sym, ok := in[0].(*lex.Symbol); ok {
		return in[1:], sym.Name, nil
	} else {
		return nil, "", ErrParse
	}
}

func lparen(in []lex.Token) ([]lex.Token, struct{}, error) { return matchOne[*lex.LParen](in) }
func rparen(in []lex.Token) ([]lex.Token, struct{}, error) { return matchOne[*lex.RParen](in) }
func kwDef(in []lex.Token) ([]lex.Token, struct{}, error)  { return matchOne[*lex.Def](in) }

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

func handleCheck() error {
	if err := recover(); err != nil {
		if err, ok := err.(*internalError); ok {
			return err.error
		} else {
			panic(err)
		}
	}

	return nil
}

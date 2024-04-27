package lex

import (
	"errors"
	"unicode"

	"github.com/horriblename/typee/src/combinator"
)

type input []rune
type output []Token

func LexString(source string) ([]Token, error) {
	parser := combinator.Many(combinator.Any(lparen, rparen, symbol))
	_, tokens, err := parser([]rune(source))

	return tokens, err
}

type ParseResult[I any, O any] struct {
	rest I
	out  O
	err  error
}

var ErrLex = errors.New("could not lex")

func lparen(in []rune) ([]rune, Token, error) {
	return combinator.MatchOne(in, '(', &LParen{})
}

func rparen(in []rune) ([]rune, Token, error) {
	return combinator.MatchOne(in, ')', &RParen{})
}

func symbol(in []rune) ([]rune, Token, error) {
	if len(in) == 0 || unicode.IsNumber(in[0]) {
		return nil, nil, ErrLex
	}

	var i int
	var char rune
	for i, char = range in {
		switch char {
		case '(', ')', '{', '}', ':':
			break
		default:
			if unicode.IsSpace(char) {
				break
			}
		}
	}

	if i == 0 {
		return nil, nil, ErrLex
	}

	return in[i:], &Symbol{
		String: string(in[:i]),
	}, nil
}

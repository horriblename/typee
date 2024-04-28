package lex

import (
	"errors"
	"unicode"

	"github.com/horriblename/typee/src/combinator"
)

type input []rune
type output []Token

func LexString(source string) ([]Token, error) {
	input := []rune(source)
	input, _, _ = skipWhitespace(input)

	token := combinator.WithSuffix(
		combinator.Any(lparen, rparen, keywordOrSymbol),
		skipWhitespace,
	)
	parser := combinator.Many(token)
	_, tokens, err := parser(input)

	return tokens, err
}

type ParseResult[I any, O any] struct {
	rest I
	out  O
	err  error
}

var ErrLex = errors.New("could not lex")

func lparen(in []rune) ([]rune, Token, error) { return combinator.MatchOne(in, '(', &LParen{}) }
func rparen(in []rune) ([]rune, Token, error) { return combinator.MatchOne(in, ')', &RParen{}) }

func keywordOrSymbol(in []rune) ([]rune, Token, error) {
	rest, symName, err := symbolStr(in)
	if err != nil {
		return nil, nil, err
	}

	switch symName {
	case "def":
		return rest, &Def{}, nil
	case "set":
		return rest, &Set{}, nil
	}

	return rest, &Symbol{Name: symName}, err
}

func symbolStr(in []rune) ([]rune, string, error) {
	if len(in) == 0 || unicode.IsNumber(in[0]) {
		return nil, "", ErrLex
	}

	var i int
	var char rune
	for i, char = range in {
		switch char {
		case '(', ')', '{', '}', ':':
			if i == 0 {
				return nil, "", ErrLex
			}
			return in[i:], string(in[:i]), nil
		}

		if unicode.IsSpace(char) {
			if i == 0 {
				return nil, "", ErrLex
			}
			return in[i:], string(in[:i]), nil
		}
	}

	// reached end of input
	return make([]rune, 0), string(in), nil
}

func skipWhitespace(in []rune) ([]rune, struct{}, error) {
	for i, char := range in {
		if !unicode.IsSpace(char) {
			return in[i:], struct{}{}, nil
		}
	}

	// reached end of input
	return make([]rune, 0), struct{}{}, nil
}

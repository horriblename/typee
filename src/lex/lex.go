package lex

import (
	"errors"
	"fmt"
	"strconv"
	"unicode"

	"github.com/horriblename/typee/src/combinator"
)

type input []rune
type output []Token

var ErrExpectEOF = errors.New("expected EOF")

func LexString(source string) ([]Token, error) {
	input := []rune(source)
	input, _, _ = skipped(input)

	token := combinator.WithSuffix(
		combinator.Any(
			lparen,
			rparen,
			lbracket,
			rbracket,
			lbrace,
			rbrace,
			comma,
			dot,
			hash,
			strLiteral,
			doubleColon,
			colon,
			tag,
			rawIdent,
			number,
			keywordOrSymbol,
		),
		skipped,
	)
	parser := combinator.Many(token)
	rest, tokens, err := parser(input)

	if len(rest) != 0 {
		return nil, fmt.Errorf("%w: found '%s'...", ErrExpectEOF, string(rest[:min(len(rest), 10)]))
	}

	return tokens, err
}

type ParseResult[I any, O any] struct {
	rest I
	out  O
	err  error
}

var ErrLex = errors.New("could not lex")

func lparen(in []rune) ([]rune, Token, error)   { return combinator.MatchOne(in, '(', &LParen{}) }
func rparen(in []rune) ([]rune, Token, error)   { return combinator.MatchOne(in, ')', &RParen{}) }
func lbracket(in []rune) ([]rune, Token, error) { return combinator.MatchOne(in, '[', &LBracket{}) }
func rbracket(in []rune) ([]rune, Token, error) { return combinator.MatchOne(in, ']', &RBracket{}) }
func lbrace(in []rune) ([]rune, Token, error)   { return combinator.MatchOne(in, '{', &LBrace{}) }
func rbrace(in []rune) ([]rune, Token, error)   { return combinator.MatchOne(in, '}', &RBrace{}) }
func colon(in []rune) ([]rune, Token, error)    { return combinator.MatchOne(in, ':', &Colon{}) }
func comma(in []rune) ([]rune, Token, error)    { return combinator.MatchOne(in, ',', &Comma{}) }
func dot(in []rune) ([]rune, Token, error)      { return combinator.MatchOne(in, '.', &Dot{}) }
func hash(in []rune) ([]rune, Token, error)     { return combinator.MatchOne(in, '#', &Hash{}) }
func doubleQuote(in []rune) ([]rune, struct{}, error) {
	return combinator.MatchOne(in, '"', struct{}{})
}

func doubleColon(in []rune) ([]rune, Token, error) {
	if len(in) < 2 || in[0] != ':' || in[1] != ':' {
		return nil, nil, ErrLex
	}
	return in[2:], &DoubleColon{}, nil
}

func strLiteral(in []rune) ([]rune, Token, error) {
	rest, strContent, err := combinator.Surround(
		doubleQuote,
		notDoubleQuote,
		doubleQuote,
	)(in)

	if err != nil {
		return nil, nil, err
	}

	return rest, &StrLiteral{Content: strContent}, err
}

func notDoubleQuote(in []rune) ([]rune, string, error) {
	// FIXME: probably some unicode bug here
	for i, c := range in {
		if c == '"' {
			return in[i:], string(in[:i]), nil
		}
	}

	return nil, "", ErrLex
}

func number(in []rune) ([]rune, Token, error) {
	var sign int64 = 1
	if len(in) > 0 && in[0] == '-' {
		sign = -1
		in = in[1:]
	}
	intPartLen := digitsLen(in)
	if intPartLen == 0 {
		return nil, nil, ErrLex
	}
	intPart := in[:intPartLen]
	rest := in[intPartLen:]
	if len(rest) == 0 || rest[0] != '.' {
		// FIXME: parse negatives as int64
		num, err := strconv.ParseUint(string(intPart), 10, 64)
		if err != nil {
			panic("failed assertion: " + err.Error())
		}
		return rest, &IntLiteral{Number: int64(num) * sign}, nil
	}

	rest = rest[1:]
	fracPartLen := digitsLen(rest)
	rest = rest[fracPartLen:]
	floatPart := in[:intPartLen+1+fracPartLen]

	num, err := strconv.ParseFloat(string(floatPart), 64)
	if err != nil {
		panic("failed assertion: " + err.Error())
	}
	return rest, &FloatLiteral{Number: num * float64(sign)}, nil
}

func digitsLen(in []rune) int {
	i := 0
	for i = 0; i < len(in); i++ {
		c := in[i]
		if c < '0' || c > '9' {
			break
		}
	}

	return i
}

func rawIdent(in []rune) ([]rune, Token, error) {
	if len(in) < 3 {
		return nil, nil, ErrLex
	}

	if in[0] != 'r' || in[1] != '@' {
		return nil, nil, ErrLex
	}

	rest, symName, err := symbolStr(in[2:])
	if err != nil {
		return nil, nil, err
	}

	return rest, &Symbol{Name: symName, Raw: true}, nil
}

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
	case "var":
		return rest, &Var{}, nil
	case "true":
		return rest, &TrueLiteral{}, nil
	case "false":
		return rest, &FalseLiteral{}, nil
	case "if":
		return rest, &If{}, nil
	case "let":
		return rest, &Let{}, nil
	case "letrec":
		return rest, &LetRec{}, nil
	case "fn":
		return rest, &Fn{}, nil
	case "case":
		return rest, &Case{}, nil
	case "interface":
		return rest, &Interface{}, nil
	case "class":
		return rest, &Class{}, nil
	case "pub":
		return rest, &Pub{}, nil
	case "protected":
		return rest, &Protected{}, nil
	case "priv":
		return rest, &Priv{}, nil
	case "self":
		return rest, &Self{}, nil
	case "Self":
		return rest, &SelfType{}, nil
	case "new":
		return rest, &New{}, nil
	case "union":
		return rest, &Union{}, nil
	case "enum":
		return rest, &Enum{}, nil
	case "type":
		return rest, &Type{}, nil
	case "callExtern":
		return rest, &CallExtern{}, nil
	case "import":
		return rest, &Import{}, nil
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
		case '(', ')', '[', ']', '{', '}', ':', ',', '.', '\'', '#':
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

func tag(in []rune) ([]rune, Token, error) {
	if len(in) == 0 || in[0] != '\'' {
		return nil, nil, ErrLex
	}

	in, label, err := symbolStr(in[1:])
	if err != nil {
		return nil, nil, err
	}

	return in, &Tag{
		Label: label,
	}, nil
}

func skipped(in []rune) ([]rune, struct{}, error) {
	rest, _, err := whitespace(in)
	if err != nil {
		return in, struct{}{}, nil
	}
	in = rest

	for {
		rest, _, err = comment(in)
		if err != nil {
			return in, struct{}{}, nil
		}
		in = rest

		rest, _, err := whitespace(in)
		if err != nil {
			return in, struct{}{}, nil
		}
		in = rest
	}
}

func whitespace(in []rune) ([]rune, struct{}, error) {
	for i, char := range in {
		if !unicode.IsSpace(char) {
			return in[i:], struct{}{}, nil
		}
	}

	return []rune{}, struct{}{}, nil
}

func comment(in []rune) ([]rune, struct{}, error) {
	if len(in) == 0 || in[0] != ';' {
		return nil, struct{}{}, ErrLex
	}

	for i, c := range in[1:] {
		// who cares about windows lmao
		if c == '\n' {
			return in[2+i:], struct{}{}, nil
		}
	}

	return []rune{}, struct{}{}, nil
}

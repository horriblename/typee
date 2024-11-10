package parse

import (
	"fmt"

	"github.com/horriblename/typee/src/combinator"
	"github.com/horriblename/typee/src/lex"
	"github.com/horriblename/typee/src/types"
)

func type_(in []lex.Token) ([]lex.Token, TypeRepr, error) {
	return combinator.Any(
		typeName,
	)(in)
}

func typeName(in []lex.Token) ([]lex.Token, TypeRepr, error) {
	return combinator.Map(symbolName, func(s string) TypeRepr {
		return TypeName{s}
	})(in)
}

func classDef(in []lex.Token) ([]lex.Token, Expr, error) {
	in, res, err := combinator.Surround(lparen,
		combinator.WithPrefix(
			kwClass,
			combinator.Then(
				symbolName,
				combinator.Then(
					combinator.Maybe(symbolName),
					combinator.Surround(
						lbrace,
						combinator.Delimited(classField, comma),
						rbrace,
					),
				),
			),
		),
		rparen)(in)

	if err != nil {
		return nil, nil, err
	}

	t := ClassDef{
		id:     newId(),
		Name:   res.One,
		Super:  res.Two.One,
		Fields: res.Two.Two,
	}
	return in, &t, nil
}

func classField(in []lex.Token) ([]lex.Token, ClassField, error) {
	in, res, err := combinator.Then(
		combinator.Maybe(memberVisibility),
		combinator.Then(
			symbolName,
			type_,
		),
	)(in)
	if err != nil {
		println("class field err", err.Error())
		return nil, ClassField{}, err
	}

	c := ClassField{
		Access: res.One.Or(types.AccessPrivate),
		Name:   res.Two.One,
		Type:   res.Two.Two,
	}
	return in, c, nil
}

func memberVisibility(in []lex.Token) ([]lex.Token, types.AccessLvl, error) {
	if len(in) == 0 {
		return nil, 0, errAt(in)
	}

	switch in[0].(type) {
	case *lex.Pub:
		return in[1:], types.AccessPublic, nil
	case *lex.Protected:
		return in[1:], types.AccessProtected, nil
	case *lex.Priv:
		return in[1:], types.AccessPrivate, nil
	default:
		return nil, 0, fmt.Errorf("not visibility token: %s", in[0].String())
	}
}

func dbg[I, O any](tag string, p combinator.Parser[I, O]) combinator.Parser[I, O] {
	return func(i I) (I, O, error) {
		r, o, e := p(i)
		if e != nil {
			println(tag, e.Error(), "at", i)
		} else {
			fmt.Printf("%s got %v\n", tag, o)
		}
		return r, o, e
	}
}

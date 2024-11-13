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
					combinator.Maybe(combinator.Surround(lparen, combinator.Many0(symbolName), rparen)),
					combinator.Surround(
						lbrace,
						combinator.Delimited(classMember, comma),
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
		Supers: res.Two.One.Or([]string{}),
		Fields: res.Two.Two,
	}
	return in, &t, nil
}

func classMember(in []lex.Token) ([]lex.Token, ClassMember, error) {
	in, vis, _ := combinator.Maybe(memberVisibility)(in)
	rest, res, err := classField(in, vis.Or(types.AccessPrivate))
	if err != nil {
		return classMethod(in, vis.Or(types.AccessPrivate))
	}

	return rest, res, nil
}

func classField(in []lex.Token, visibility types.AccessLvl) ([]lex.Token, ClassField, error) {
	in, res, err := combinator.Then(
		symbolName,
		type_,
	)(in)
	if err != nil {
		return nil, ClassField{}, err
	}

	return in, ClassField{
		Access_: visibility,
		Name_:   res.One,
		Type:    res.Two,
	}, nil
}

func classMethod(in []lex.Token, visibility types.AccessLvl) ([]lex.Token, ClassMember, error) {
	in, res, err := defForm(in)
	if err != nil {
		return nil, nil, err
	}

	return in, ClassMethod{
		Access_: visibility,
		Func:    res,
	}, nil
}

func interfaceDef(in []lex.Token) ([]lex.Token, Expr, error) {
	in, res, err := combinator.Surround(lparen,
		combinator.WithPrefix(
			kwInterface,
			combinator.Then(
				symbolName,
				combinator.Then(
					combinator.Maybe(combinator.Surround(lparen, combinator.Many0(symbolName), rparen)),
					combinator.Surround(
						lbrace,
						combinator.Delimited(classMember, comma),
						rbrace,
					),
				),
			),
		),
		rparen)(in)

	if err != nil {
		return nil, nil, err
	}

	t := InterfaceDef{
		id:     newId(),
		Name:   res.One,
		Supers: res.Two.One.Or([]string{}),
		Fields: res.Two.Two,
	}
	return in, &t, nil
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
			fmt.Printf("[dbg] %s: %s at %v\n", tag, e.Error(), i)
		} else {
			fmt.Printf("[dbg] %s got %v\n", tag, o)
		}
		return r, o, e
	}
}

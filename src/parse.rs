use nom::branch::alt;
use nom::combinator::map;
use nom::sequence::delimited;
use nom::{combinator::all_consuming, multi::many0, IResult};
use nom_locate::LocatedSpan;

use crate::lex::Token;
use crate::ast;

type PResult<'a, O> = IResult<&'a [Token<'a>], O>;

// pub fn parse_string<'a>(source: &'a str) -> PResult<'a, ast::Program> {
//     parse_tokens(&crate::lex::lex(source).expect.1);
// }

pub fn parse_tokens<'a>(tokens: &'a [Token]) -> PResult<'a, ast::Program<'a>> {
    many0(parse_expr)(tokens)
}

fn parse_expr<'a>(source: &'a [Token]) -> PResult<'a, ast::Expr<'a>> {
    alt((
        parse_list,
        parse_symbol,
        parse_int_literal,
    ))(source)
}

fn parse_list<'a>(source: &'a [Token]) -> PResult<'a, ast::Expr<'a>> {
    map(
        delimited(lparen, many0(parse_expr), rparen),
        |exprs| ast::Expr::List(exprs)
    )(source)
}

fn lparen<'a>(source: &'a [Token]) -> PResult<'a, ()> {
    let (i, o) = one(source)?;
    match o {
        Token::LParen(_) => Ok((i, ())),
        _ => temp_error(i),
    }
}

fn rparen<'a>(source: &'a [Token]) -> PResult<'a, ()> {
    let (i, o) = one(source)?;
    match o {
        Token::RParen(_) => Ok((i, ())),
        _ => temp_error(i),
    }
}

fn parse_symbol<'a>(source: &'a [Token]) -> PResult<'a, ast::Expr<'a>> {
    let (i, o) = one(source)?;
    let Token::Symbol(span) = o else {
        return temp_error(i);
    };

    Ok((i, ast::Expr::Symbol(span.fragment())))
}

fn parse_int_literal<'a>(source: &'a[Token]) -> PResult<'a, ast::Expr<'a>> {
    let (i, o) = one(source)?;
    let Token::IntLiteral(_, num) = o else {
        return temp_error(i);
    };

    Ok((i, ast::Expr::IntLiteral(*num)))
}

fn one<'a>(source: &'a [Token]) -> PResult<'a, &'a Token<'a>> {
    match source.first() {
        Some(tok) => {
            Ok((&source[1..], tok))
        }
        _ => temp_error(source),
    }
}

fn temp_error<'a, O>(source: &'a[Token]) -> PResult<'a, O> {
    Err(nom::Err::Error(nom::error::make_error(
        source,
        nom::error::ErrorKind::IsNot,
    )))
}

#[cfg(test)]
mod tests {
    use crate::lex;
    use crate::ast::Expr;

    use super::parse_tokens;

    #[test]
    fn test_ast() {
        use Expr::*;

        let input = "(def hi (x y) (foo 2 x))";
        let (_, tokens) = lex::lex(input).expect("lex error");
        let (_, ast) = parse_tokens(&tokens).expect("parse error");

        let expect = vec![
            List(vec![
                 Symbol("def"),
                 Symbol("hi"),
                 List(vec![Symbol("x"), Symbol("y")]),
                 List(vec![Symbol("foo"), IntLiteral(2), Symbol("x")]),
            ])
        ];

        assert_eq!(ast, expect);
    }
}

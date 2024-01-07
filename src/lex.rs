use nom::{combinator::{all_consuming, map, value}, multi::many0, IResult, sequence::{delimited, pair}, character::complete::multispace1, bytes::complete::{take_until, take_while, take_while1, tag}, branch::alt};
use nom_locate::{LocatedSpan, position};



#[derive(Debug)]
pub enum Token<'a> {
    LParen(Span<'a>),
    RParen(Span<'a>),
    Symbol(Span<'a>),
    IntLiteral(Span<'a>, i32),
}

type Span<'a> = LocatedSpan<&'a str>;

pub fn lex<'a>(source: &'a str) -> IResult<Span<'a>, Vec<Token<'a>>> {
    lex_span(source.into())
}

fn lex_span<'a>(source: Span<'a>) -> IResult<Span<'a>, Vec<Token<'a>>> {
    all_consuming(many0(delimited(lex_ignored, alt ((
        map(tag("("), Token::LParen),
        map(tag(")"), Token::RParen),
        lex_number,
        lex_symbol,
    )), lex_ignored)))(source)
}

fn lex_ignored<'a>(source: Span<'a>) -> IResult<Span<'a>, ()> {
    value(
        (),
        many0(alt((
                    value((), multispace1),
                    value(
                        (),
                        pair(
                            tag(";"),
                            alt((take_until("\n"), take_while(|_| true)))
                            )
                        )
                  ))),
        )(source)
}

fn lex_number<'a>(source: Span<'a>) ->  IResult<Span<'a>, Token> {
    map(
        nom::sequence::pair(position, nom::character::complete::i32),
        |(pos, n)| Token::IntLiteral(pos, n)
    )(source)
}

fn lex_symbol<'a>(source: Span<'a>) -> IResult<Span<'a>, Token> {
    let (rest, output) = take_while1(is_symbol)(source)?;

    Ok((rest, Token::Symbol(output)))
}

fn is_symbol(c: char)-> bool {
    match c {
        '(' | ')' | '"' | ' ' | '\n' | '\t' | '\r' => false,
        _ => true,
    }
}

#[cfg(test)]
mod tests {
    use super::{lex, Token::{self, *}};

    #[test]
    fn test_tokens() {
        let (_, got) = lex("(foo (+ 3))").unwrap();
        let expected = vec![
            LParen("(".into()),
            Symbol("foo".into()),
            LParen("(".into()),
            Symbol("+".into()),
            IntLiteral("3".into(), 3),
            RParen(")".into()),
            RParen(")".into()),
        ]; 

        assert_eq!(got.len(), expected.len());
        for (got, exp) in got.iter().zip(&expected) {
            assert!(token_eq(got, exp));
        }
    }

    fn token_eq(left: &Token, right: &Token) -> bool {
        match (left, right) {
            (LParen(_), LParen(_)) => true,
            (RParen(_), RParen(_)) => true,
            (Symbol(s1), Symbol(s2)) => s1.fragment() == s2.fragment(),
            (IntLiteral(_, n1), IntLiteral(_, n2)) => n1 == n2,
            _ => false
        }
    }
}

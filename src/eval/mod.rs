mod scope;
mod object;
pub mod error;

use crate::{ast, parse::parse_tokens, lex::lex};
use std::rc::Rc;
use object::{Object};
use error::Error;

struct State<'a> {
    scopes: scope::ScopeStack<'a>,
}

impl State<'_> {
    fn new<'a>() -> State<'a> {
        State{scopes: scope::ScopeStack::new()}
    }
}

type EResult<O> = Result<O, Error>;

pub fn eval_program(source: &str) {
    let (_, tokens) = lex(source).expect("TODO: lex error");
    let (_, ast) = parse_tokens(&tokens).expect("TODO: parse error");
    println!("{:?}", eval_ast(&mut State::new(), &ast))
}

pub fn eval_ast<'a>(state: &mut State<'a>, program: &'a ast::Program<'a>) -> EResult<Object<'a>> {
    let mut last_return: Option<Object> = None;
    for expr in program {
        last_return = Some(eval_expr(state, expr)?);
    }

    Ok(last_return.unwrap_or(Object::Nil))
}

fn eval_expr<'a>(state: &'_ mut State<'a>, expr: &'a ast::Expr<'a>) -> EResult<Object<'a>> {
    match expr {
        ast::Expr::List(body) => eval_list(state, body),
        ast::Expr::Symbol(name) => state.scopes.find(name).ok_or(Error::UnboundVar(name.to_string())),
        ast::Expr::IntLiteral(n) => Ok(Object::Int(*n)),
    }
}

fn eval_list<'a>(state: &mut State<'a>, body: &'a [ast::Expr]) -> EResult<Object<'a>> {
    let mut body_iter = body.iter();
    let name = body_iter.next().ok_or(Error::EmptyListForm)?;

    match eval_expr(state, name)? {}
}


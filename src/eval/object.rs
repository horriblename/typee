use std::rc::Rc;

use crate::ast;

#[derive(Clone, Debug)]
pub enum Object<'a> {
    Int(i32),
    Nil,
    Boxed(Rc<Boxed<'a>>),
}

pub enum Boxed<'a> {
    Func(ast::Program<'a>)
}

impl std::fmt::Debug for Boxed<'_> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> Result<(), std::fmt::Error> {
        match self {
            Self::Func(_) => write!(f, "function_object"),
        }
    }
}


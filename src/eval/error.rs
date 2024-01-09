use std::fmt::{Display, Formatter};

#[derive(Debug)]
pub enum Error {
    UnboundVar(String),
    EmptyListForm,
}

impl Display for Error {
    fn fmt(&self, f: &mut Formatter<'_>) -> Result<(), std::fmt::Error> {
        match self {
            Self::UnboundVar(var_name) => write!(f, "Unbound variable `{}`", var_name),
            Self::EmptyListForm => write!(f, ""),
        }
    }
}

impl std::error::Error for Error {}

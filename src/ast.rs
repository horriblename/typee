pub type Program<'a> = Vec<Expr<'a>>;

#[derive(Debug, PartialEq)]
pub enum Expr<'a> {
    List(Vec<Expr<'a>>),
    Symbol(&'a str),
    IntLiteral(i32),
}

interface Parse exposes []
    imports [
        parc.Parser.{Parser},
        parc.Byte,
        parc.Combinator.{matches, many0, complete, surrounded},
        Lex.{Token},
        Debug,
    ]


## A node in the Abstract Syntax Tree
Expr : [Form (List Expr), Symbol Str, Int I32]

lparen = matches LParen
rparen = matches RParen

# form : Parser (List Token) Expr
form = surrounded lparen (many0 expr) rparen
    |> Parser.map Form


# expr : Parser (List Token) Expr
expr = \input ->
    when input is
        [LParen, ..] -> form input
        [Symbol sym, .. as rest] -> Ok (rest, Symbol sym)
        [IntLiteral num, .. as rest] -> Ok (rest, Int num)
        _ -> Err Parser.genericError

program = many0 expr |> complete

parseTokens : List Token -> Result (List Expr) Parser.Problem
parseTokens = \input ->
    (_, prog) <- Result.map (program input)
    prog

parseSource : Str -> Result (List Expr) Parser.Problem
parseSource = \source ->
    source
        |> Lex.lex
        |> Result.try parseTokens

expect
    Debug.expectEql
        (parseSource "(+ 1 y)")
        (Ok [Form [Symbol "+", Int 1, Symbol "y"]])

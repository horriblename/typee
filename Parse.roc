interface Parse exposes [Expr, Program, parseTokens, parseStr, parse]
    imports [
        parc.Parser.{ Parser },
        parc.Combinator.{ matches, many0, prefixed, surrounded, andThen },
        Lex.{ Token },
        Debug,
    ]

## A node in the Abstract Syntax Tree
Expr : [
    Form (List Expr),
    Symbol Str,
    Int I32,
    FunctionDef
        {
            name : Str,
            args : List Str,
            body : Expr,
        },
    Set { name : Str, rvalue : Expr },
]
Program : List Expr

lparen = matches LParen
rparen = matches RParen
kwDef = matches Def
kwSet = matches Set
parenthesized = \token -> surrounded lparen token rparen

symName =
    symbol
    |> Parser.try \tok ->
        when tok is
            Symbol name -> Ok name
            _ -> Err Parser.genericError

functionDefinition : Parser (List Token) Expr
functionDefinition =

    argDef = parenthesized (many0 symName)
    formBody =
        kwDef
        |> prefixed symName
        |> andThen argDef
        |> andThen expr
        |> Parser.map \((name, args), body) ->
            FunctionDef { name, args, body }

    parenthesized formBody

setStatement : Parser (List Token) Expr
setStatement =
    parenthesized
        (
            kwSet
            |> prefixed symName
            |> andThen expr
        )
    |> Parser.map \(name, rvalue) -> Set { name, rvalue }

symbol =
    \tokens ->
        when tokens is
            [Symbol name, .. as rest] -> Ok (rest, Symbol name)
            _ -> Err Parser.genericError

# form : Parser (List Token) Expr
form =
    surrounded lparen (many0 expr) rparen
    |> Parser.map Form

# expr : Parser (List Token) Expr
expr = \input ->
    when input is
        [LParen, Def, ..] -> functionDefinition input
        [LParen, Set, ..] -> setStatement input
        [LParen, ..] -> form input
        [Symbol sym, .. as rest] -> Ok (rest, Symbol sym)
        [IntLiteral num, .. as rest] -> Ok (rest, Int num)
        _ -> Err Parser.genericError

program = many0 expr

parseTokens : List Token -> Result (List Expr) Parser.Problem
parseTokens = \input ->
    Parser.complete program input

parseStr : Str -> Result (List Expr) Parser.Problem
parseStr = \source ->
    source
    |> Lex.lexStr
    |> Result.try parseTokens

parse : List U8 -> Result (List Expr) Parser.Problem
parse = \source ->
    source
    |> Lex.lex
    |> Result.try parseTokens

expect
    Debug.expectEql
        (parseStr "(+ 1 y)")
        (Ok [Form [Symbol "+", Int 1, Symbol "y"]])

expect
    Debug.expectEql
        (parseStr "(set foo (+ 1 x))")
        (
            Ok [
                Set {
                    name: "foo",
                    rvalue: Form [Symbol "+", Int 1, Symbol "x"],
                },
            ]
        )

expect
    Debug.expectEql
        (parseStr "(def foo (bar) (+ bar 1))")
        (
            Ok [
                FunctionDef {
                    name: "foo",
                    args: ["bar"],
                    body: Form [Symbol "+", Symbol "bar", Int 1],
                },
            ]

        )

interface Parse exposes [Expr, Program, parseTokens, parseStr, parse]
    imports [
        parc.Parser.{ Parser },
        parc.Combinator.{ matches, many0, prefixed, surrounded, andThen, passes },
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
]
Program : List Expr

lparen = matches LParen
rparen = matches RParen
parenthesized = \token -> surrounded lparen token rparen

functionDefinition : Parser (List Token) Expr
functionDefinition =
    symName =
        symbol
        |> Parser.try \tok ->
            when tok is
                Symbol name -> Ok name
                _ -> Err Parser.genericError

    argDef = parenthesized (many0 symName)
    formBody =
        keyword "def"
        |> prefixed symName
        |> andThen argDef
        |> andThen expr
        |> Parser.map \((name, args), body) ->
            FunctionDef { name, args, body }

    parenthesized formBody

# FIXME: symbols must not be keywords
symbol =
    \tokens ->
        when tokens is
            [Symbol name, .. as rest] -> Ok (rest, Symbol name)
            _ -> Err Parser.genericError

keyword = \name ->
    passes \token ->
        when token is
            Symbol str if str == name -> Bool.true
            _ -> Bool.false

# form : Parser (List Token) Expr
form =
    surrounded lparen (many0 expr) rparen
    |> Parser.map Form

# expr : Parser (List Token) Expr
expr = \input ->
    when input is
        [LParen, Symbol "def", ..] -> functionDefinition input
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

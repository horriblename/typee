module [Expr, Program, parseTokens, parseStr, parse]

import parc.Parser exposing [Parser]
import parc.Combinator exposing [matches, many, many0, prefixed, suffixed, surrounded, andThen]
import Lex exposing [Token]
import Debug

## A node in the Abstract Syntax Tree
Expr : [
    Form (List Expr),
    Symbol Str,
    Int I32,
    StrLit Str,
    FunctionDef
        {
            name : Str,
            args : List Str,
            body : Expr,
        },
    Set { name : Str, rvalue : Expr },
    Record { members : List { key : Str, value : Expr } },
    Do (List Expr),
]
Program : List Expr

lparen = matches LParen
rparen = matches RParen
lbrace = matches LBrace
rbrace = matches RBrace
colon = matches Colon
kwDef = matches Def
kwSet = matches Set
kwDo = matches Do
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

doStatement : Parser (List Token) Expr
doStatement =
    parenthesized
        (
            kwDo
            |> prefixed (many expr)
        )
    |> Parser.map \exprs -> Do exprs

symbol =
    \tokens ->
        when tokens is
            [Symbol name, .. as rest] -> Ok (rest, Symbol name)
            _ -> Err Parser.genericError

# form : Parser (List Token) Expr
form =
    surrounded lparen (many0 expr) rparen
    |> Parser.map Form

record : Parser (List Token) Expr
record =
    entry : Parser (List Token) { key : Str, value : Expr }
    entry =
        symName
        |> suffixed colon
        |> andThen expr
        |> Parser.map \(key, value) -> { key, value }

    surrounded lbrace (many0 entry) rbrace
    |> Parser.map \members -> Record { members }

# expr : Parser (List Token) Expr
expr = \input ->
    when input is
        [LParen, Def, ..] -> functionDefinition input
        [LParen, Set, ..] -> setStatement input
        [LParen, Do, ..] -> doStatement input
        [LParen, ..] -> form input
        [LBrace, ..] -> record input
        [Symbol sym, .. as rest] -> Ok (rest, Symbol sym)
        [StrLiteral str, .. as rest] -> Ok (rest, StrLit str)
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
        (parseStr "(do (+ 1 2) (+ 3 4))")
        (Ok [Do [Form [Symbol "+", Int 1, Int 2], Form [Symbol "+", Int 3, Int 4]]])

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

expect
    Debug.expectEql
        (parseStr "(\"hello world\" 2)")
        (Ok [Form [StrLit "hello world", Int 2]])

# expect
#     Debug.expectEql
#         (parseStr "{x: 3}")
#         (Ok [Form [Record { members: [{ key: "x", value: Int 3 }] }]])

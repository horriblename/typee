interface Lex exposes [lex, lexStr]
    imports [
        parc.Parser,
        parc.Parser.{ Parser },
        parc.Ascii.{ char, StrBuf, isDigit, int, charIs, tag },
        parc.Combinator.{ prefixed, suffixed, many0, alt, andThen },
        Bool.{ true, false },
        Debug,
    ]

Token : [
    LParen,
    RParen,
    Symbol Str,
    IntLiteral I32,

    # Keywords
    Def,
    Set,
]

lparen : Parser StrBuf Token
lparen =
    char '('
    |> Parser.map \_ -> LParen

rparen : Parser StrBuf Token
rparen =
    char ')'
    |> Parser.map \_ -> RParen

symbol : Parser StrBuf Token
symbol =
    str = charIs identFirst |> andThen (many0 (charIs identBody))
    (first, body) <- Parser.try str
    body
    |> List.prepend first
    |> Str.fromUtf8
    |> Result.mapErr \_ -> Parser.genericError
    |> Result.map Symbol

number : Parser StrBuf Token
number =
    int
    |> Parser.map \numStr ->
        numRes =
            numStr
            |> Str.fromUtf8
            |> Result.try Str.toI32

        when numRes is
            Ok num -> IntLiteral num
            _ -> crash "unreachable"

identFirst = \c -> c != '(' && c != ')' && !(isWhitespace c) && !(isDigit c)
identBody = \c -> c != '(' && c != ')' && !(isWhitespace c)

isWhitespace = \c ->
    when c is
        0x0020 | 0x000A | 0x000D | 0x0009 -> Bool.true
        _ -> Bool.false

# keywords
kwDef = tag "def" |> Parser.map \_ -> Def
kwSet = tag "set" |> Parser.map \_ -> Set

# whitespace = tag " "
# skipWhitespaces : Parser StrBuf {}
skipWhitespaces = \input ->
    count =
        input
        |> List.walkUntil 0 \i, c ->
            if
                isWhitespace c
            then
                Continue (i + 1)
            else
                Break i

    rest = List.dropFirst input count
    Ok (rest, {})

lex : List U8 -> Result (List Token) Parser.Problem
lex = \input ->
    token : Parser StrBuf Token
    token =
        alt [lparen, rparen, kwDef, kwSet, symbol, number]
        |> suffixed skipWhitespaces

    parser : Parser StrBuf (List Token)
    parser =
        skipWhitespaces
        |> prefixed (many0 token)

    Parser.complete parser input

lexStr = \input -> lex (Str.toUtf8 input)

expect
    Debug.expectEql
        (lexStr "(hi 4)")
        (Ok [LParen, Symbol "hi", IntLiteral 4, RParen])

expect
    Debug.expectEql
        (lexStr "def foo set")
        (Ok [Def, Symbol "foo", Set])

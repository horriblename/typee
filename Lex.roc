interface Lex exposes [lex, lexStr]
    imports [
        parc.Parser,
        parc.Parser.{ Parser },
        parc.Ascii.{ char, StrBuf, isDigit, int, charIs, isWhitespace, until },
        parc.Combinator.{ prefixed, suffixed, many0, alt, andThen, surrounded, opt },
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
    Do,
]

lparen : Parser StrBuf Token
lparen =
    char '('
    |> Parser.map \_ -> LParen

rparen : Parser StrBuf Token
rparen =
    char ')'
    |> Parser.map \_ -> RParen

symbolStr : Parser StrBuf Str
symbolStr =
    str = charIs identFirst |> andThen (many0 (charIs identBody))
    (first, body) <- Parser.try str
    body
    |> List.prepend first
    |> Str.fromUtf8
    |> Result.mapErr \_ -> Parser.genericError

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

# isWhitespace = \c ->
#     when c is
#         0x0020 | 0x000A | 0x000D | 0x0009 -> Bool.true
#         _ -> Bool.false

keywordOrSymbol =
    symbolStr
    |> Parser.map \sym ->
        when sym is
            "def" -> Def
            "set" -> Set
            "do" -> Do
            _ -> Symbol sym

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

whitespaces = until \c -> !(isWhitespace c)

alternate0 : Parser i o, Parser i o -> Parser i (List o)
alternate0 = \p1, p2 -> \in ->
        inner = \pi1, pi2, input, outputs ->
            when pi1 input is
                Ok (rest, out) ->
                    inner pi2 pi1 rest (List.append outputs out)

                Err _ -> Ok (input, outputs)

        inner p1 p2 in []

lift = \result, onErr, onOk ->
    when result is
        Ok ok -> onOk ok
        Err err -> onErr err

comment = \input ->
    when input is
        [';', ..] ->
            { before, after } <- List.splitFirst input '\n'
                |> lift \_ -> Ok ([], input)

            Ok (after, before)

        _ -> Err Parser.genericError

skipWhitespacesAndComments =
    alternate0
        (opt whitespaces |> Parser.map \_ -> {})
        (comment |> Parser.map \_ -> {})

lex : List U8 -> Result (List Token) Parser.Problem
lex = \input ->
    token : Parser StrBuf Token
    token =
        alt [lparen, rparen, keywordOrSymbol, number]
        |> suffixed skipWhitespacesAndComments

    parser : Parser StrBuf (List Token)
    parser =
        skipWhitespacesAndComments
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

expect
    Debug.expectEql
        (lexStr "def define")
        (Ok [Def, Symbol "define"])

expect
    Debug.expectEql
        (lexStr "(hi ; comment \n); comment 2")
        (Ok [LParen, Symbol "hi", RParen])

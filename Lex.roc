interface Lex exposes [lex, lexStr]
    imports [
        parc.Parser,
        parc.Parser.{ Parser },
        parc.Ascii.{ char, StrBuf, isDigit, int, charIs, isWhitespace, until, until0 },
        parc.Combinator.{ prefixed, suffixed, many0, alt, andThen, surrounded, opt },
        Bool.{ true, false },
        Debug,
    ]

Token : [
    LParen,
    RParen,
    LBrace,
    RBrace,
    Colon,
    Symbol Str,
    IntLiteral I32,
    StrLiteral Str,

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

lbrace = char '{' |> Parser.map \_ -> LBrace
rbrace = char '}' |> Parser.map \_ -> RBrace
colon = char ':' |> Parser.map \_ -> Colon

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

# TODO: uninclude {}
identFirst = \c -> identBody c && !(isDigit c)
identBody = \c ->
    when c is
        '(' | ')' | '{' | '}' | ':' -> Bool.false
        _ if isWhitespace c -> Bool.false
        _ -> Bool.true

keywordOrSymbol =
    symbolStr
    |> Parser.map \sym ->
        when sym is
            "def" -> Def
            "set" -> Set
            "do" -> Do
            _ -> Symbol sym

strLiteral =
    doubleQuote = char '"'

    surrounded doubleQuote (until0 \c -> c == '"') doubleQuote
    |> Parser.try \utf8 ->
        Str.fromUtf8 utf8
        |> Result.mapErr \_ -> Parser.genericError
    |> Parser.map StrLiteral

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
        alt [lparen, rparen, lbrace, rbrace, colon, strLiteral, keywordOrSymbol, number]
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

expect
    Debug.expectEql
        (lexStr "hi \"hello\"")
        (Ok [Symbol "hi", StrLiteral "hello"])

expect
    Debug.expectEql
        (lexStr "hi\"hi\"")
        (Ok [Symbol "hi", StrLiteral "hi"])
    |> Debug.expectFail

expect
    Debug.expectEql
        (lexStr "{foo:bar}")
        (Ok [LBrace, Symbol "foo", Colon, Symbol "bar", RBrace])

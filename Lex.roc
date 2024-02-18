interface Lex exposes [lex]
    imports [
        parc.Parser,
        parc.Parser.{ Parser },
        parc.Byte.{ char, strToRaw, RawStr, isAsciiDigit, int },
        parc.Combinator.{ prefixed, suffixed, many0, alt, complete },
        Bool.{true, false},
        Debug
    ]

Token : [LParen, RParen, Symbol RawStr, IntLiteral I32]

lparen : Parser RawStr Token
lparen = char '('
    |> Parser.map \_ -> LParen

rparen : Parser RawStr Token
rparen = char ')' 
    |> Parser.map \_ -> RParen


symbol : Parser RawStr Token
symbol = \input ->
    when input is
        [first, .. as rest] if identFirst first ->
            len = List.walkUntil rest 1 \i, c ->
                if identBody c then
                    Continue (i+1)
                else
                    Break i

            {before, others} = List.split input len 
            Ok (others, Symbol before)
        _ -> Err Parser.genericError

number : Parser RawStr Token
number = int
    |> Parser.map \numStr -> 
        numRes = numStr
            |> Str.fromUtf8
            |> Result.try Str.toI32

        when numRes is
            Ok num -> IntLiteral num
            _ -> crash "unreachable"

identFirst = \c -> c != '(' && c != ')' && !(isWhitespace c) && !(isAsciiDigit c)
identBody = \c -> c != '(' && c != ')' && !(isWhitespace c)

isWhitespace = \c ->
    when c is
        0x0020 | 0x000A | 0x000D | 0x0009 -> Bool.true
        _ -> Bool.false

# whitespace = tag " "
# skipWhitespaces : Parser RawStr {}
skipWhitespaces = \input ->
    count = input |> List.walkUntil 0 \i, c ->
        if isWhitespace c
        then Continue (i+1)
        else Break i

    rest = List.dropFirst input count
    Ok (rest, {})

lex : Str -> Result (List Token) Parser.Problem
lex = \input ->
    rawInput = strToRaw input

    token : Parser RawStr Token
    token = alt [lparen, rparen, symbol, number]
        |> suffixed skipWhitespaces

    parser : Parser RawStr (List Token)
    parser = skipWhitespaces
        |> prefixed (many0 token)
        |> complete

    (_, out) <- Result.try (Parser.run parser rawInput)
    Ok out

expect 
    Debug.expectEql
        (lex "(hi 4)") 
        (Ok [LParen, Symbol ['h', 'i'], IntLiteral 4, RParen])

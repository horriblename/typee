interface Backend.StackVm.OpCode
    exposes [OpCode, fromNum, toNum]
    imports []

OpCode : [
    Halt,
    Push,
    Add,
    Sub,
    Mul,
    Div,
    Not,
    And,
    Or,
    Pop,
    Dup,
    ## `Jmp addr` jumps to an address
    Jmp,
    ## Conditional `Jmp`: `Jif addr` jumps to the address if the value on the stack is truthy
    Jif,
]

# rename to fromInstruction?
fromNum : Num * -> Result OpCode [NotFound]
fromNum = \num ->
    when num is
        0 -> Ok Halt
        1 -> Ok Push
        2 -> Ok Add
        3 -> Ok Sub
        4 -> Ok Mul
        5 -> Ok Div
        6 -> Ok Not
        7 -> Ok And
        8 -> Ok Or
        9 -> Ok Pop
        10 -> Ok Dup
        11 -> Ok Jmp
        12 -> Ok Jif
        _ -> Err NotFound

toNum : OpCode -> Num *
toNum = \opcode ->
    when opcode is
        Halt -> 0
        Push -> 1
        Add -> 2
        Sub -> 3
        Mul -> 4
        Div -> 5
        Not -> 6
        And -> 7
        Or -> 8
        Pop -> 9
        Dup -> 10
        Jmp -> 11
        Jif -> 12

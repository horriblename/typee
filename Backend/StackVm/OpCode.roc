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
    ## Next word should be the variable number (id of the variable)
    ## pushes the variable value onto the stack
    Load,
    ## Next word should be the variable number (id of the variable)
    ## pops a value and store into the given variable
    Store,
    Call,
    Ret,
    # Temporary OpCode to make my life easier
    # prints the top item on the stack
    Print,
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
        13 -> Ok Load
        14 -> Ok Store
        15 -> Ok Call
        16 -> Ok Ret
        17 -> Ok Print
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
        Load -> 13
        Store -> 14
        Call -> 15
        Ret -> 16
        Print -> 17

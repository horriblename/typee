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

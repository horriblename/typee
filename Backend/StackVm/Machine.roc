interface Backend.StackVm.Machine
    exposes [Machine, new, run]
    imports [
        Backend.StackVm.OpCode.{OpCode, fromNum, toNum},
    ]

Instr: Nat

RuntimeProblem : [
    InvalidProgram [
        UnknownInstruction,
        PushAtEndOfProgram,
        StepAtEndOfProgram,
    ],

    ## not enough items on stack
    NotEnoughOperands,
]

MachineInner : {
    program: List Instr,
    stack: List Nat,
    instructionAddr: Nat,
    halted: Bool,
}
Machine := MachineInner

new : List Nat -> Machine
new = \instructions ->
    if List.len instructions < 0 then
        crash "A program should have at least an instruction"
    else
        @Machine {
            program: instructions,
            stack: [],
            instructionAddr: 0,
            halted: Bool.false,
        }

run : Machine -> Result Machine RuntimeProblem
run = \@Machine self ->
    if self.halted then
        Ok (@Machine self)
    else
        step (@Machine self)
            |> Result.try run

step = \@Machine self ->
    (mach, instr) <- nextWord self
        |> Result.mapErr \_ -> InvalidProgram StepAtEndOfProgram
        |> Result.try
    dbg StepNextInstr instr
    decodeInstruction (@Machine mach) instr

decodeInstruction : Machine, Nat -> Result Machine RuntimeProblem
decodeInstruction = \@Machine self, instruction ->
    popTwo = \mach ->
        {} <- checkStackItemCount (@Machine mach) 2 |> Result.try

        (mach1, n1) <- popStack (@Machine mach) |> Result.try
        (mach2, n2) <- popStack (mach1) |> Result.try

        Ok (mach2, n1, n2)

    when fromNum instruction is
        Ok Halt ->
            Ok (@Machine {self & halted: Bool.true})
        Ok Push ->
            (self1, value) <- nextWord self
                |> Result.mapErr \_ -> InvalidProgram PushAtEndOfProgram
                |> Result.try
            Ok (@Machine {
                self1 &
                stack: List.append self.stack value,
            })

        Ok Add ->
            (mach1, n1, n2) <- Result.try (popTwo self)
            Ok (push mach1 (n1+n2))

        Ok Sub ->
            (mach1, n1, n2) <- Result.try (popTwo self)
            Ok (push mach1 (n2 - n1))

        Ok Mul ->
            (mach1, n1, n2) <- Result.try (popTwo self)
            Ok (push mach1 (n1*n2))

        Ok Div ->
            (mach1, n1, n2) <- Result.try (popTwo self)
            Ok (push mach1 (n2 // n1))

        Ok Not ->
            (mach1, n1) <- popStack (@Machine self)
                |> Result.mapErr \_ -> NotEnoughOperands
                |> Result.try

            Ok (mach1 |> push (if n1 == 0 then 1 else 0))

        Err _ -> Err (InvalidProgram UnknownInstruction)

and = \res, pred ->
    when res is
        Ok ok -> pred ok
        Err _ -> Bool.false

runAndCheck = \instr, pred ->
    result = new instr
        |> run
    when result is
        Ok (@Machine mach) -> pred mach
        Err err ->
            dbg ExecuteError err
            crash "error running code"

expect
    runAndCheck
        [toNum Push, 1, toNum Halt]
        \mach -> tag (mach.stack) FinalStack == [1]

expect
    runAndCheck
        [toNum Push, 1, toNum Push, 2, toNum Add, toNum Halt]
        \mach -> tag (mach.stack) FinalStack == [3]

expect
    runAndCheck
        [toNum Push, 3, toNum Push, 2, toNum Sub, toNum Halt]
        \mach -> tag (mach.stack) FinalStack == [1]

expect
    runAndCheck
        [toNum Push, 2, toNum Push, 3, toNum Mul, toNum Halt]
        \mach -> tag (mach.stack) FinalStack == [6]

expect
    runAndCheck
        [toNum Push, 7, toNum Push, 3, toNum Div, toNum Halt]
        \mach -> tag (mach.stack) FinalStack == [2]

expect
    runAndCheck
        [toNum Push, 0, toNum Not, toNum Halt]
        \mach -> tag (mach.stack) FinalStack == [1]

checkState : ({} -> Bool) -> Result {} [CheckStateFailed]
checkState = \test ->
    if test {} then
        Ok {}
    else
        Err CheckStateFailed

checkStackItemCount = \@Machine mach, atLeast ->
    if List.len mach.stack >= atLeast then
        Ok {}
    else
        Err NotEnoughOperands

nextWord : MachineInner -> Result (MachineInner, Instr) [EndOfProgram]
nextWord = \self ->
    dbg StartNextWord self
    word <- List.get self.program self.instructionAddr 
        |> tag NextWordResult
        |> Result.mapErr \_ -> EndOfProgram
        |> Result.try
    Ok (
        {self & instructionAddr: self.instructionAddr + 1},
        word
    )

popStack = \@Machine self ->
    when self.stack is
        [.. as stack, last] -> Ok (@Machine {self & stack: stack}, last)
        _ -> Err NotEnoughOperands

push = \@Machine self, item ->
    @Machine {self & stack: List.append self.stack item}

tag = \x, f ->
    dbg (f x)
    x

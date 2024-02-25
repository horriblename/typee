interface Backend.StackVm.Machine
    exposes [Machine, new, run]
    imports [
        # NOTE: compiler bug made it so that I can't use functions by Module.func
        Backend.StackVm.OpCode.{ OpCode, fromNum, toNum },
        Backend.StackVm.Frame.{ Frame, empty, getVariable, setVariable },
        Debug,
    ]

Instr : U64

RuntimeProblem : [
    InvalidProgram
        [
            UnknownInstruction,
            PushAtEndOfProgram,
            JmpAtEndOfProgram,
            JifAtEndOfProgram,
            StepAtEndOfProgram,
            JumpOutOfProgram,
            LoadAtEndOfProgram,
            StoreAtEndOfProgram,
            UnknownVariable U64,
            StoreMissingValueOnStack,
        ],

    ## not enough items on stack
    NotEnoughOperands,
]

MachineInner : {
    program : List Instr,
    stack : List U64,
    instructionAddr : U64,
    halted : Bool,
    currentFrame : Frame,
}
Machine := MachineInner

new : List U64 -> Machine
new = \instructions ->
    if List.len instructions < 0 then
        crash "A program should have at least an instruction"
    else
        @Machine {
            program: instructions,
            stack: [],
            instructionAddr: 0,
            halted: Bool.false,
            currentFrame: empty {},
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

decodeInstruction : Machine, U64 -> Result Machine RuntimeProblem
decodeInstruction = \@Machine self, instruction ->
    popTwo = \mach ->
        {} <- checkStackItemCount (@Machine mach) 2 |> Result.try

        (mach1, n1) <- popStack (@Machine mach) |> Result.try
        (mach2, n2) <- popStack (mach1) |> Result.try

        Ok (mach2, n1, n2)

    when fromNum instruction is
        Ok Halt ->
            Ok (@Machine { self & halted: Bool.true })

        Ok Push ->
            (self1, value) <- nextWord self
                |> Result.mapErr \_ -> InvalidProgram PushAtEndOfProgram
                |> Result.try
            Ok
                (
                    @Machine
                        { self1 &
                            stack: List.append self.stack value,
                        }
                )

        Ok Add ->
            (mach1, n1, n2) <- Result.try (popTwo self)
            Ok (push mach1 (n1 + n2))

        Ok Sub ->
            (mach1, n1, n2) <- Result.try (popTwo self)
            Ok (push mach1 (n2 - n1))

        Ok Mul ->
            (mach1, n1, n2) <- Result.try (popTwo self)
            Ok (push mach1 (n1 * n2))

        Ok Div ->
            (mach1, n1, n2) <- Result.try (popTwo self)
            Ok (push mach1 (n2 // n1))

        Ok Not ->
            (mach1, n1) <- popStack (@Machine self)
                |> Result.mapErr \_ -> NotEnoughOperands
                |> Result.try

            Ok (mach1 |> push (if n1 == 0 then 1 else 0))

        Ok And ->
            (mach1, n1, n2) <- Result.try (popTwo self)
            Ok (push mach1 (if n1 != 0 && n2 != 0 then 1 else 0))

        Ok Or ->
            (mach1, n1, n2) <- Result.try (popTwo self)
            Ok (push mach1 (if n1 != 0 || n2 != 0 then 1 else 0))

        Ok Pop ->
            (mach1, _) <- popStack (@Machine self) |> Result.try
            Ok mach1

        Ok Dup ->
            last <- List.last self.stack
                |> Result.mapErr \_ -> NotEnoughOperands
                |> Result.try
            Ok (push (@Machine self) last)

        Ok Jmp ->
            (mach1, addr) <- nextWord self
                |> Result.mapErr \_ -> InvalidProgram JmpAtEndOfProgram
                |> Result.try
            if addr >= List.len self.program then
                Err (InvalidProgram JumpOutOfProgram)
            else
                Ok (@Machine { mach1 & instructionAddr: addr })

        Ok Jif ->
            (mach1, addr) <- nextWord self
                |> Result.mapErr \_ -> InvalidProgram JifAtEndOfProgram
                |> Result.try

            cond <- topOfStack (@Machine self)
                |> Result.mapErr \_ -> NotEnoughOperands
                |> Result.try

            if addr >= List.len self.program then
                Err (InvalidProgram JumpOutOfProgram)
            else if cond != 0 then
                Ok (@Machine { mach1 & instructionAddr: addr })
            else
                Ok (@Machine mach1)

        Ok Load ->
            (self1, varNumber) <- nextWord self
                |> Result.mapErr \_ -> InvalidProgram LoadAtEndOfProgram
                |> Result.try

            value <- getVariable self1.currentFrame varNumber
                |> Result.mapErr \_ -> InvalidProgram (UnknownVariable varNumber)
                |> Result.try

            push (@Machine self1) value |> Ok

        Ok Store ->
            (self1, varNumber) <- nextWord self
                |> Result.mapErr \_ -> InvalidProgram StoreAtEndOfProgram
                |> Result.try

            (mach2, value) <- popStack (@Machine self1)
                |> Result.mapErr \_ -> InvalidProgram StoreMissingValueOnStack
                |> Result.try

            setVar mach2 varNumber value |> Ok

        Err _ -> Err (InvalidProgram UnknownInstruction)

runAndCheck = \instr, pred ->
    result =
        new instr
        |> run
    when result is
        Ok (@Machine mach) -> pred mach
        Err err ->
            dbg ExecuteError err

            crash "error running code"

expect
    runAndCheck
        [toNum Push, 1, toNum Halt]
        \mach -> mach.stack == [1]

expect
    runAndCheck
        [toNum Push, 1, toNum Push, 2, toNum Add, toNum Halt]
        \mach -> mach.stack == [3]

expect
    runAndCheck
        [toNum Push, 3, toNum Push, 2, toNum Sub, toNum Halt]
        \mach -> mach.stack == [1]

expect
    runAndCheck
        [toNum Push, 2, toNum Push, 3, toNum Mul, toNum Halt]
        \mach -> mach.stack == [6]

expect
    runAndCheck
        [toNum Push, 7, toNum Push, 3, toNum Div, toNum Halt]
        \mach -> mach.stack == [2]

expect
    runAndCheck
        [toNum Push, 0, toNum Not, toNum Halt]
        \mach -> mach.stack == [1]

expect
    runAndCheck
        [toNum Push, 1, toNum Push, 0, toNum And, toNum Halt]
        \mach -> mach.stack == [0]

expect
    runAndCheck
        [toNum Push, 0, toNum Push, 1, toNum Or, toNum Halt]
        \mach -> mach.stack == [1]

expect
    runAndCheck
        [toNum Push, 0, toNum Push, 1, toNum Pop, toNum Halt]
        \mach -> mach.stack == [0]

expect
    runAndCheck
        [toNum Push, 0, toNum Dup, toNum Halt]
        \mach -> mach.stack == [0, 0]

expect
    runAndCheck
        [toNum Jmp, 3, toNum Halt, toNum Jmp, 2]
        \mach -> mach.instructionAddr == 3

expect
    runAndCheck
        [toNum Push, 1, toNum Jif, 5, toNum Pop, toNum Push, 0, toNum Jif, 4, toNum Halt]
        \mach -> mach.instructionAddr == 10

# store variable
expect
    runAndCheck
        [toNum Push, 42, toNum Store, 0, toNum Halt]
        \mach -> Debug.expectEql mach.instructionAddr 5
            && Debug.expectEql mach.stack []
            && Debug.expectEql (getVariable mach.currentFrame 0) (Ok 42)

# store and load variable
expect
    runAndCheck
        [toNum Push, 42, toNum Store, 0, toNum Load, 0, toNum Halt]
        \mach -> Debug.expectEql mach.instructionAddr 7
            && Debug.expectEql mach.stack [42]
            && Debug.expectEql (getVariable mach.currentFrame 0) (Ok 42)

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
        { self & instructionAddr: self.instructionAddr + 1 },
        word,
    )

popStack = \@Machine self ->
    when self.stack is
        [.. as stack, last] -> Ok (@Machine { self & stack: stack }, last)
        _ -> Err NotEnoughOperands

push = \@Machine self, item ->
    @Machine { self & stack: List.append self.stack item }

getVar = \@Machine self, varNumber -> getVariable self.currentFrame varNumber
setVar = \@Machine self, varNumber, newValue ->
    newFrame = setVariable self.currentFrame varNumber newValue

    @Machine { self & currentFrame: newFrame }

topOfStack = \@Machine self ->
    List.last self.stack

tag = \x, f ->
    dbg f x

    x

module [Machine, new, run, Instr]

# NOTE: compiler bug made it so that I can't use functions by Module.func
import pf.Stdout
import pf.Task exposing [Task, await]
import Backend.StackVm.OpCode exposing [OpCode, fromNum, toNum]
import Backend.StackVm.Frame exposing [Frame, empty, getVariable, setVariable, returnAddr]
import Backend.StackVm.NonEmptyStack exposing [NonEmptyStack, single, updateLast, last, pushNES, popNES]
import Debug

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
            CallAtEndOfProgram,
            UnknownVariable U64,
            StoreMissingValueOnStack,
            AttemptToPopLastFrame,
            BadReturnAddress,
        ],

    ## not enough items on stack
    NotEnoughOperands,
]

MachineInner : {
    program : List Instr,
    stack : List U64,
    instructionAddr : U64,
    halted : Bool,
    frames : NonEmptyStack Frame,
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
            # TODO: return addr of main?
            frames: single (empty 0),
        }

run : Machine -> Task Machine RuntimeProblem
run = \self ->
    if halted self then
        Task.ok self
    else
        when step self is
            Ok (self1, NoTask) -> run self1
            Ok (self1, WithTask task) ->
                result <- await task
                run self1

            Err err -> Task.err err

## For testing only, like run but crashes when a Task is encountered
runPure : Machine -> Result Machine RuntimeProblem
runPure = \self ->
    if halted self then
        Ok self
    else
        when step self is
            Ok (self1, NoTask) -> runPure self1
            Ok (_, WithTask _) -> crash "runPure encountered a Task"
            Err err -> Err err

step = \@Machine self ->
    (mach, instr) <- nextWord self
        |> Result.mapErr \_ -> InvalidProgram StepAtEndOfProgram
        |> Result.try
    dbg StepNextInstr (fromNum instr) instr

    decodeInstruction (@Machine mach) instr

halted = \@Machine self -> self.halted

# To make testing a little easier, I decided to make Tasks "optional"
decodeInstruction : Machine, U64 -> Result (Machine, [NoTask, WithTask (Task Machine RuntimeProblem)]) RuntimeProblem
decodeInstruction = \@Machine self, instruction ->
    popTwo = \mach ->
        {} <- checkStackItemCount (@Machine mach) 2 |> Result.try

        (mach1, n1) <- popStack (@Machine mach) |> Result.try
        (mach2, n2) <- popStack (mach1) |> Result.try

        Ok (mach2, n1, n2)

    when fromNum instruction is
        Ok Halt ->
            Ok (@Machine { self & halted: Bool.true }, NoTask)

        Ok Push ->
            (self1, value) <- nextWord self
                |> Result.mapErr \_ -> InvalidProgram PushAtEndOfProgram
                |> Result.try
            Ok (
                @Machine
                    { self1 &
                        stack: List.append self.stack value,
                    },
                NoTask,
            )

        Ok Add ->
            (mach1, n1, n2) <- Result.try (popTwo self)
            Ok (push mach1 (n1 + n2), NoTask)

        Ok Sub ->
            (mach1, n1, n2) <- Result.try (popTwo self)
            Ok (push mach1 (n2 - n1), NoTask)

        Ok Mul ->
            (mach1, n1, n2) <- Result.try (popTwo self)
            Ok (push mach1 (n1 * n2), NoTask)

        Ok Div ->
            (mach1, n1, n2) <- Result.try (popTwo self)
            Ok (push mach1 (n2 // n1), NoTask)

        Ok Not ->
            (mach1, n1) <- popStack (@Machine self)
                |> Result.mapErr \_ -> NotEnoughOperands
                |> Result.try

            Ok (mach1 |> push (if n1 == 0 then 1 else 0), NoTask)

        Ok And ->
            (mach1, n1, n2) <- Result.try (popTwo self)
            Ok (push mach1 (if n1 != 0 && n2 != 0 then 1 else 0), NoTask)

        Ok Or ->
            (mach1, n1, n2) <- Result.try (popTwo self)
            Ok (push mach1 (if n1 != 0 || n2 != 0 then 1 else 0), NoTask)

        Ok Pop ->
            (mach1, _) <- popStack (@Machine self) |> Result.try
            Ok (mach1, NoTask)

        Ok Dup ->
            lastItem <- List.last self.stack
                |> Result.mapErr \_ -> NotEnoughOperands
                |> Result.try
            Ok (push (@Machine self) lastItem, NoTask)

        Ok Jmp ->
            (mach1, addr) <- nextWord self
                |> Result.mapErr \_ -> InvalidProgram JmpAtEndOfProgram
                |> Result.try

            checkJumpAddress (@Machine mach1) addr
            |> Result.map \{} ->
                (@Machine { mach1 & instructionAddr: addr }, NoTask)

        # if addr >= List.len self.program then
        #     Err (InvalidProgram JumpOutOfProgram)
        # else
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
                Ok (@Machine { mach1 & instructionAddr: addr }, NoTask)
            else
                Ok (@Machine mach1, NoTask)

        Ok Load ->
            (self1, varNumber) <- nextWord self
                |> Result.mapErr \_ -> InvalidProgram LoadAtEndOfProgram
                |> Result.try

            value <- getVar (@Machine self1) varNumber
                |> Result.mapErr \_ -> InvalidProgram (UnknownVariable varNumber)
                |> Result.try

            Ok (push (@Machine self1) value, NoTask)

        Ok Store ->
            (self1, varNumber) <- nextWord self
                |> Result.mapErr \_ -> InvalidProgram StoreAtEndOfProgram
                |> Result.try

            (mach2, value) <- popStack (@Machine self1)
                |> Result.mapErr \_ -> InvalidProgram StoreMissingValueOnStack
                |> Result.try

            Ok (setVar mach2 varNumber value, NoTask)

        Ok Call ->
            (self1, callAddress) <- nextWord self
                |> Result.mapErr \_ -> InvalidProgram CallAtEndOfProgram
                |> Result.try
            returnAddress = self1.instructionAddr

            {} <- checkJumpAddress (@Machine self1) callAddress |> Result.try
            (@Machine self2) = pushFrame (@Machine self1) (empty returnAddress)

            Ok (@Machine { self2 & instructionAddr: callAddress }, NoTask)

        Ok Ret ->
            {} <- checkReturnAddressExists (@Machine self) |> Result.try

            returnAddress = currReturnAddr (@Machine self)
            (mach1, _) <- popFrame (@Machine self) |> Result.try
            Ok (updateInstructionAddr mach1 returnAddress, NoTask)

        Ok Print ->
            task =
                topOfStack (@Machine self)
                |> Result.mapErr \_ -> NotEnoughOperands
                |> Result.map Num.toStr
                |> Task.fromResult
                |> await \str ->
                    Stdout.line str
                    |> await \{} -> Task.ok (@Machine self)
            Ok (@Machine self, WithTask task)

        Err _ -> Err (InvalidProgram UnknownInstruction)

runAndCheck = \instr, pred ->
    result =
        new instr
        |> runPure
    when result is
        Ok (@Machine mach) -> pred mach
        Err err ->
            dbg err

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
            && Debug.expectEql (getVar (@Machine mach) 0) (Ok 42)

# store and load variable
expect
    runAndCheck
        [toNum Push, 42, toNum Store, 0, toNum Load, 0, toNum Halt]
        \mach -> Debug.expectEql mach.instructionAddr 7
            && Debug.expectEql mach.stack [42]
            && Debug.expectEql (getVar (@Machine mach) 0) (Ok 42)

# function call no args no return
expect
    runAndCheck
        [toNum Call, 3, toNum Halt, toNum Ret]
        \mach -> Debug.expectEql mach.instructionAddr 3
            && Debug.expectEql mach.stack []

# function call no args returns int
expect
    runAndCheck
        [toNum Call, 3, toNum Halt, toNum Push, 7, toNum Ret]
        \mach -> Debug.expectEql mach.instructionAddr 3
            && Debug.expectEql mach.stack [7]

# function doubles givene argument
expect
    runAndCheck
        [toNum Push, 3, toNum Call, 5, toNum Halt, toNum Push, 2, toNum Mul, toNum Ret]
        \mach -> Debug.expectEql mach.instructionAddr 5
            && Debug.expectEql mach.stack [6]

checkStackItemCount = \@Machine mach, atLeast ->
    if List.len mach.stack >= atLeast then
        Ok {}
    else
        Err NotEnoughOperands

checkJumpAddress = \@Machine self, addr ->
    if addr >= List.len self.program then
        Err (InvalidProgram JumpOutOfProgram)
    else
        Ok {}

checkReturnAddressExists = \@Machine self ->
    if (last self.frames |> returnAddr) == 0 then
        Err (InvalidProgram BadReturnAddress)
    else
        Ok {}

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
        [.. as stack, lastItem] -> Ok (@Machine { self & stack: stack }, lastItem)
        _ -> Err NotEnoughOperands

push = \@Machine self, item ->
    @Machine { self & stack: List.append self.stack item }

pushFrame = \@Machine self, frame ->
    @Machine { self & frames: pushNES self.frames frame }

popFrame = \@Machine self ->
    (frames1, popped) <- popNES self.frames
        |> Result.mapErr \_ -> InvalidProgram AttemptToPopLastFrame
        |> Result.try
    Ok (@Machine { self & frames: frames1 }, popped)

currReturnAddr = \@Machine self ->
    last self.frames |> returnAddr

updateInstructionAddr = \@Machine self, address ->
    @Machine { self & instructionAddr: address }

getVar = \@Machine self, varNumber -> last self.frames |> getVariable varNumber
setVar = \@Machine self, varNumber, newValue ->
    frames1 = self.frames |> updateLast \frame -> setVariable frame varNumber newValue

    @Machine { self & frames: frames1 }

topOfStack = \@Machine self ->
    List.last self.stack

tag = \x, f ->
    dbg f x

    x

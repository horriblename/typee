module [assemble, compileFromSource, compileFromAsciiSource]

import Debug
import Backend.StackVm.CodeGen exposing [Assembly, asmInstr, AsmInstr, genAssemblyFromStr, genAssemblyFromAscii]
import Backend.StackVm.Machine exposing [Instr]
import Backend.StackVm.OpCode exposing [toNum]

Assembler : {
    # TODO: Resolved/Missing is not used, remove?
    labelTable : Dict Str [Resolved U64, Missing],
    code : List [Code U64, UnresolvedLabel Str],
}

Problem : [
    LabelRedefined Str,
    UnresolvedLabel Str,
]

new = \{} -> {
    labelTable: Dict.empty {},
    code: [],
}

currAddr : Assembler -> U64
currAddr = \self -> List.len self.code

assemble : Assembly -> Result (List Instr) Problem
assemble = \asm ->
    assembler0 = new {}
    assemblerUnresolved = List.walkTry asm assembler0 \assembler, instr ->

        when instr is
            OpCode code -> assembler |> appendCode (toNum code) |> Ok
            Raw code -> assembler |> appendCode code |> Ok
            Label labelName -> assembler |> appendLabel labelName |> Ok
            LabelDef labelName -> resolveLabel assembler labelName

    assemblerUnresolved |> Result.try finish

compileFromSource = \source ->
    genAssemblyFromStr source
    |> Result.try \asm ->
        assemble asm |> Result.mapErr Assembler

compileFromAsciiSource = \source ->
    genAssemblyFromAscii source
    |> Result.try \asm ->
        assemble asm |> Result.mapErr Assembler

appendCode = \self, code -> { self & code: List.append self.code (Code code) }
appendLabel = \self, labelName -> { self & code: List.append self.code (UnresolvedLabel labelName) }

resolveLabel : Assembler, Str -> Result Assembler Problem
resolveLabel = \self, name ->
    ensureUnresolved =
        when Dict.get self.labelTable name is
            Ok (Resolved _) -> Err (LabelRedefined name)
            _ -> Ok {}
    {} <- Result.try ensureUnresolved

    labelTable = Dict.insert self.labelTable name (Resolved (currAddr self))

    Ok { self & labelTable }

## Resolves all labels and outputs a `List Instr` byte code program
finish : Assembler -> Result (List Instr) [UnresolvedLabel Str]
finish = \self ->
    List.mapTry self.code \byte ->
        when byte is
            Code code -> Ok code
            UnresolvedLabel label ->
                maybeAddr <- Dict.get self.labelTable label
                    |> Result.mapErr \_ -> UnresolvedLabel label
                    |> Result.try

                when maybeAddr is
                    Missing -> Err (UnresolvedLabel label)
                    Resolved addr -> Ok addr

expect
    asm = [
        asmInstr (OpCode Call),
        asmInstr (Label "foo"),
        asmInstr (OpCode Halt),
        LabelDef "foo",
        asmInstr (OpCode Push),
        asmInstr (Raw 42),
        asmInstr (OpCode Ret),
    ]

    assemble asm
    |> Debug.okAnd \code ->
        code
        == [
            toNum Call,
            3,
            toNum Halt,
            toNum Push,
            42,
            toNum Ret,
        ]

interface Backend.StackVm.Assembler
    exposes [assemble, compileFromSource, compileFromAsciiSource]
    imports [
        Debug,
        Backend.StackVm.CodeGen.{ Assembly, asmInstr, AsmInstr, genAssemblyFromStr, genAssemblyFromAscii },
        Backend.StackVm.Machine.{ Instr },
        Backend.StackVm.OpCode.{ toNum },
    ]

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
        assembler1 <- resolveLabel assembler instr (currAddr assembler) |> Result.map

        when instr.instr is
            OpCode code -> assembler1 |> appendCode (toNum code)
            Raw code -> assembler1 |> appendCode code
            Label labelName -> assembler1 |> appendLabel labelName

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

LabelResolutionProblem : [
    LabelRedefined Str,
]

resolveLabel : Assembler, AsmInstr, U64 -> Result Assembler LabelResolutionProblem
resolveLabel = \self, { label }, addr ->
    when label is
        None -> Ok self
        Some name ->
            ensureUnresolved =
                when Dict.get self.labelTable name is
                    Ok (Resolved _) -> Err (LabelRedefined name)
                    _ -> Ok {}
            {} <- Result.try ensureUnresolved

            labelTable = Dict.insert self.labelTable name (Resolved addr)

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
        asmInstr { instr: OpCode Call },
        asmInstr { instr: Label "foo" },
        asmInstr { instr: OpCode Halt },
        asmInstr { label: Some "foo", instr: OpCode Push },
        asmInstr { instr: Raw 42 },
        asmInstr { instr: OpCode Ret },
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

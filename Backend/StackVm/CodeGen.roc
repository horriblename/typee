interface Backend.StackVm.CodeGen
    exposes [genAssembly, asmInstr, Assembly, AsmInstr]
    imports [
        Parse.{ Expr, parseStr },
        Debug,
        Backend.StackVm.Machine.{ Instr },
        Backend.StackVm.OpCode.{ OpCode },
    ]

AssemblyBuilder := {
    symbolsInCurrentTable : U64,
    localSymbolTable : Dict Str U64,
    instructions : Assembly,
}

BuildProblem : [EmptyFormOrBadFunctionName, WrongArgCount]

## An intermediate representation of the final byte code, with labels that are resolved by the assembler
Assembly : List AsmInstr

AsmInstr : {
    label : [None, Some Str],
    instr : [
        OpCode OpCode,
        Raw Instr,
        Label Str,
    ],
}

asmInstr : { label ? [None, Some Str], instr : [OpCode OpCode, Raw Instr, Label Str] } -> AsmInstr
asmInstr = \{ label ? None, instr } -> { label, instr }

newBuilder = \{} -> @AssemblyBuilder {
        symbolsInCurrentTable: 0,
        localSymbolTable: Dict.empty {},
        instructions: [],
    }

finish : AssemblyBuilder -> Assembly
finish = \@AssemblyBuilder builder -> builder.instructions

genAssembly : List Expr -> Result Assembly BuildProblem
genAssembly = \program ->
    List.walkUntil program (Ok (newBuilder {})) \buiderRes, expr ->
        when buiderRes is
            Err err -> Break (Err err)
            Ok builder ->
                when genForExpr builder { expr } is
                    Err err -> Break (Err err)
                    Ok ok -> Continue (Ok ok)
    |> Result.map finish

genForExpr : AssemblyBuilder, { expr : Expr, label ? [None, Some Str] } -> Result AssemblyBuilder BuildProblem
genForExpr = \self, { expr, label ? None } ->
    when expr is
        Form form -> self |> genCall form
        Symbol _ -> crash ""
        Int n ->
            self
            |> addPushInstr (Num.toU64 n)
            |> Ok

        FunctionDef { name, args, body } ->
            self
            |> genArgList args
            |> genForExpr { expr: body, label: Some name }

genArgList : AssemblyBuilder, List Str -> AssemblyBuilder
genArgList = \self, args ->
    List.walk args self \asmBuilder, arg ->
        (builder1, varNum) = uninitializedVariable asmBuilder arg
        addStoreInstr builder1 varNum

uninitializedVariable : AssemblyBuilder, Str -> (AssemblyBuilder, U64)
uninitializedVariable = \@AssemblyBuilder self, name ->
    (
        @AssemblyBuilder
            { self &
                symbolsInCurrentTable: self.symbolsInCurrentTable + 1,
                localSymbolTable: Dict.insert self.localSymbolTable name self.symbolsInCurrentTable,
            },
        self.symbolsInCurrentTable,
    )

genCall : AssemblyBuilder, List Expr -> Result AssemblyBuilder BuildProblem
genCall = \@AssemblyBuilder self, formBody ->
    when formBody is
        [Symbol name, .. as args] ->
            when genCallBuiltin (@AssemblyBuilder self) name args is
                Found result -> result
                NotFound ->
                    genCallUserFunction self name args

        _ -> Err EmptyFormOrBadFunctionName

genCallBuiltin : AssemblyBuilder, Str, List Expr -> [NotFound, Found (Result AssemblyBuilder BuildProblem)]
genCallBuiltin = \self, name, args ->
    when name is
        "+" -> genBinaryOperator self Add args |> Found
        "-" -> genBinaryOperator self Sub args |> Found
        "*" -> genBinaryOperator self Mul args |> Found
        "/" -> genBinaryOperator self Div args |> Found
        "println" -> genUnaryOperator self Print args |> Found
        _ -> NotFound

genBinaryOperator = \self, opCode, args ->
    (arg1, arg2) <- Result.try (twoArgs args)
    self1 <- self
        |> genForExpr { expr: arg1 }
        |> Result.try

    self2 <- self1
        |> genForExpr { expr: arg2 }
        |> Result.try

    self2
    |> appendInstr (opCodeInstr opCode)
    |> Ok

genUnaryOperator = \self, opCode, args ->
    argResult =
        when args is
            [arg] -> Ok arg
            _ -> Err WrongArgCount
    arg <- argResult |> Result.try

    self1 <- genForExpr self { expr: arg } |> Result.try

    self1
    |> appendInstr (opCodeInstr opCode)
    |> Ok

genCallUserFunction = \self, name, args ->
    crash ""

twoArgs : List Expr -> Result (Expr, Expr) [WrongArgCount]
twoArgs = \args ->
    when args is
        [arg1, arg2] -> Ok (arg1, arg2)
        _ -> Err WrongArgCount

appendInstr = \@AssemblyBuilder self, instr ->
    @AssemblyBuilder { self & instructions: List.append self.instructions instr }

expect
    exprs <- parseStr "1" |> Debug.okAnd
    asm <- genAssembly exprs |> Debug.okAnd
    Debug.expectEql asm (addPushInstr (newBuilder {}) 1 |> finish)

opCodeInstr = \code -> { label: None, instr: OpCode code }
rawInstr = \code -> { label: None, instr: Raw code }

addPushInstr = \@AssemblyBuilder self, val ->
    instructions =
        List.append self.instructions (opCodeInstr Push)
        |> List.append (rawInstr val)

    @AssemblyBuilder { self & instructions }

addStoreInstr = \@AssemblyBuilder self, varNum ->
    instructions =
        List.append self.instructions (opCodeInstr Store)
        |> List.append (rawInstr varNum)

    @AssemblyBuilder { self & instructions }

gen : List Expr -> List Instr
gen = \exprs ->
    # genAssembly exprs
    # |> assemblyToByteCode
    crash ""


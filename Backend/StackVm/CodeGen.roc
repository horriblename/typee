interface Backend.StackVm.CodeGen
    exposes [genAssembly, genAssemblyFromStr, genAssemblyFromAscii, asmInstr, Assembly, AsmInstr, dump]
    imports [
        parc.Parser,
        Parse.{ Expr, parseStr, parse },
        Debug,
        Backend.StackVm.Machine.{ Instr },
        Backend.StackVm.OpCode.{ OpCode },
    ]

AssemblyBuilder := {
    symbolsInCurrentTable : U64,
    localSymbolTable : Dict Str U64,
    instructions : Assembly,
}

BuildProblem : [
    EmptyFormOrBadFunctionName,
    WrongArgCount,
    UndeclaredVariable,
]

## An intermediate representation of the final byte code, with labels that are resolved by the assembler
Assembly : List AsmInstr

dump = \asm ->
    dumpInstr = \{ label, instr } ->
        prefix =
            when label is
                None -> "\t\t"
                Some name -> "\(name)\t"
        data =
            when instr is
                OpCode opcode -> Inspect.toStr opcode
                Raw code -> Num.toStr code
                Label name -> "LABEL \(name)"

        Str.concat prefix data

    List.map asm dumpInstr
    |> Str.joinWith "\n"

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
finish = \@AssemblyBuilder builder -> List.append builder.instructions (opCodeInstr Halt)

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
        Symbol varName ->
            lookupSymbol self varName
            |> Result.mapErr \_ -> UndeclaredVariable
            |> Result.map \varNum -> self |> addLoadInstr varNum

        Int n ->
            self
            |> addPushInstr (Num.toU64 n)
            |> Ok

        FunctionDef { name, args, body } ->
            self
            |> genArgList args
            |> genForExpr { expr: body, label: Some name }

        Set { name, rvalue } ->
            (self1, varNum) = getOrDeclareVariableNum self name

            self1
            |> genForExpr { expr: rvalue }
            |> Result.map \self2 -> addStoreInstr self2 varNum

genArgList : AssemblyBuilder, List Str -> AssemblyBuilder
genArgList = \self, args ->
    List.walk args self \asmBuilder, arg ->
        (builder1, varNum) = declareVariable asmBuilder arg
        addStoreInstr builder1 varNum

getOrDeclareVariableNum = \self, varName ->
    when lookupSymbol self varName is
        Ok varNum -> (self, varNum)
        Err KeyNotFound -> declareVariable self varName

lookupSymbol = \@AssemblyBuilder self, varName ->
    Dict.get self.localSymbolTable varName

declareVariable : AssemblyBuilder, Str -> (AssemblyBuilder, U64)
declareVariable = \@AssemblyBuilder self, name ->
    expect
        lookupSymbol (@AssemblyBuilder self) name == Err KeyNotFound

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
    crash "TODO"

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

addLoadInstr = \@AssemblyBuilder self, varNum ->
    instructions =
        List.append self.instructions (opCodeInstr Load)
        |> List.append (rawInstr varNum)

    @AssemblyBuilder { self & instructions }

genAssemblyFromStr : Str -> Result Assembly [Parser Parser.Problem, CodeGen BuildProblem]
genAssemblyFromStr = \source ->
    parseStr source
    |> Result.mapErr Parser
    |> Result.try \ast -> genAssembly ast
        |> Result.mapErr CodeGen

genAssemblyFromAscii : List U8 -> Result Assembly [Parser Parser.Problem, CodeGen BuildProblem]
genAssemblyFromAscii = \source ->
    parse source
    |> Result.mapErr Parser
    |> Result.try \ast -> genAssembly ast
        |> Result.mapErr CodeGen

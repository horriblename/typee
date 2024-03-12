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
    globalFunctions : Dict Str Assembly,
    symbolsInCurrentTable : U64,
    localSymbolTable : Dict Str U64,
    instructions : Assembly,
}

BuildProblem : [
    EmptyFormOrBadFunctionName,
    WrongArgCount,
    UndeclaredVariable Str,
    VariableRedefined Str,
]

## An intermediate representation of the final byte code, with labels that are to be resolved by the assembler
Assembly : List AsmInstr

dump = \asm ->
    dumpInstr = \instr ->
        when instr is
            OpCode opcode -> "\t\(Inspect.toStr opcode)"
            Raw code -> "\t\(Num.toStr code)"
            Label name -> "\tLABEL \(name)"
            LabelDef name -> "\(name):"

    List.map asm dumpInstr
    |> Str.joinWith "\n"

AsmInstr : [
    OpCode OpCode,
    Raw Instr,
    Label Str,
    LabelDef Str,
]

asmInstr : AsmInstr -> AsmInstr
asmInstr = \i -> i

newBuilder = \{} -> @AssemblyBuilder {
        globalFunctions: Dict.empty {},
        symbolsInCurrentTable: 0,
        localSymbolTable: Dict.empty {},
        instructions: [],
    }

finish : AssemblyBuilder -> Assembly
finish = \@AssemblyBuilder builder ->
    Dict.values builder.globalFunctions
    |> List.join
    |> \globalFuncs -> List.join [
            [OpCode Call, Label "main", OpCode Halt],
            globalFuncs,
            builder.instructions,
        ]

genAssembly : List Expr -> Result Assembly BuildProblem
genAssembly = \program ->
    newBuilder {}
    |> genExprs program
    |> Result.map finish

genExprs = \self, exprs ->
    List.walkTry exprs self \builder, expr ->
        genForExpr builder expr

# TODO: deprecate label
genForExpr : AssemblyBuilder, Expr -> Result AssemblyBuilder BuildProblem
genForExpr = \self, expr ->
    when expr is
        Form form -> self |> genCall form
        Symbol varName ->
            lookupSymbol self varName
            |> Result.mapErr \_ -> UndeclaredVariable varName
            |> Result.map \varNum -> self |> addLoadInstr varNum

        Int n ->
            self
            |> addPushInstr (Num.toU64 n)
            |> Ok

        FunctionDef { name, args, body } ->
            (self1, varNum) <- declareVariableChecked self name
                |> Result.try

            (self2, oldTable) = self1 |> swapSymbolTable (Dict.empty {})
            swapAsm = \@AssemblyBuilder inner, asm ->
                (@AssemblyBuilder { inner & instructions: asm }, inner.instructions)
            insertGlobal = \@AssemblyBuilder inner, key, value ->
                expect
                    Dict.get inner.globalFunctions key == Err KeyNotFound

                globalFunctions = Dict.insert inner.globalFunctions key value
                @AssemblyBuilder { inner & globalFunctions }

            # HACK: couldn't be assed to rework stuff so here's a lil hack to swap the assembly and swap it back later
            (self3, parentAsm) = swapAsm self2 []

            self3
            |> addInstr (LabelDef name)
            |> genDefArgList args
            |> genForExpr body
            |> Result.map \self4 ->
                (self5, _) = swapSymbolTable self4 oldTable
                (self6, funcAsm) =
                    self5
                    |> addInstr (OpCode Ret)
                    |> swapAsm parentAsm

                insertGlobal self6 name funcAsm

        Set { name, rvalue } ->
            (self1, varNum) = getOrDeclareVariableNum self name

            self1
            |> genForExpr rvalue
            |> Result.map \self2 -> addStoreInstr self2 varNum

genDefArgList : AssemblyBuilder, List Str -> AssemblyBuilder
genDefArgList = \self, args ->
    List.walk args self \asmBuilder, arg ->
        (builder1, varNum) = declareVariable asmBuilder arg
        addStoreInstr builder1 varNum

getOrDeclareVariableNum = \self, varName ->
    when lookupSymbol self varName is
        Ok varNum -> (self, varNum)
        Err KeyNotFound -> declareVariable self varName

lookupSymbol = \@AssemblyBuilder self, varName ->
    Dict.get self.localSymbolTable varName

## declare a variable and returns its variableNumber, if the variable already exists, returns an error
declareVariableChecked = \self, name ->
    # TODO: use Dict.contatins
    lookupSymbol self name
    |> Result.try \_ -> Err (VariableRedefined name)
    |> Result.onErr \_ -> Ok (declareVariable self name)

declareVariable : AssemblyBuilder, Str -> (AssemblyBuilder, U64)
declareVariable = \@AssemblyBuilder self, name ->
    expect
        Debug.expectEql (lookupSymbol (@AssemblyBuilder self) name) (Err KeyNotFound)

    (
        @AssemblyBuilder
            { self &
                symbolsInCurrentTable: self.symbolsInCurrentTable + 1,
                localSymbolTable: Dict.insert self.localSymbolTable name self.symbolsInCurrentTable,
            },
        self.symbolsInCurrentTable,
    )

genCall : AssemblyBuilder, List Expr -> Result AssemblyBuilder BuildProblem
genCall = \self, formBody ->
    when formBody is
        [Symbol name, .. as args] ->
            when genCallBuiltin self name args is
                Found result -> result
                NotFound ->
                    genCallUserFunction self name args

        _ -> Err EmptyFormOrBadFunctionName

expect
    compileAndTest "(foo 1)" \asm ->
        Debug.expectEql asm [
            asmInstr (OpCode Push),
            asmInstr (Raw 1),
            asmInstr (OpCode Call),
            asmInstr (Label "foo"),
            asmInstr (OpCode Halt),
        ]

genCallBuiltin : AssemblyBuilder, Str, List Expr -> [NotFound, Found (Result AssemblyBuilder BuildProblem)]
genCallBuiltin = \self, name, args ->
    when name is
        "+" -> genBinaryOperator self Add args |> Found
        "-" -> genBinaryOperator self Sub args |> Found
        "*" -> genBinaryOperator self Mul args |> Found
        "/" -> genBinaryOperator self Div args |> Found
        "println" -> genUnaryOperator self Print args |> Found
        "die" -> addInstr self (OpCode Halt) |> Ok |> Found
        _ -> NotFound

expect
    compileAndTest "(die)" \asm ->
        Debug.expectEql asm [OpCode Halt, OpCode Halt]

genBinaryOperator = \self, opCode, args ->
    (arg1, arg2) <- Result.try (twoArgs args)
    self1 <- self
        |> genForExpr arg1
        |> Result.try

    self2 <- self1
        |> genForExpr arg2
        |> Result.try

    self2
    |> appendInstr (OpCode opCode)
    |> Ok

genUnaryOperator = \self, opCode, args ->
    argResult =
        when args is
            [arg] -> Ok arg
            _ -> Err WrongArgCount
    arg <- argResult |> Result.try

    self1 <- genForExpr self arg |> Result.try

    self1
    |> appendInstr (OpCode opCode)
    |> Ok

genCallUserFunction = \self, name, args ->
    # TODO: do top-level functions properly
    # TODO: should probably use separate symbolTable for functions
    # _ <- lookupSymbol self name
    #    |> Result.mapErr \_ -> UndeclaredVariable name
    #    |> Result.try

    # TODO: arity check
    self1 <- args
        |> List.reverse
        |> List.walkTry self \currSelf, arg ->
            genForExpr currSelf arg
        |> Result.try

    Ok (self1 |> addCallInstr name)

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

addInstr = \@AssemblyBuilder self, instruction ->
    instructions = List.append self.instructions instruction
    @AssemblyBuilder { self & instructions }

addPushInstr = \@AssemblyBuilder self, val ->
    instructions =
        List.append self.instructions (OpCode Push)
        |> List.append (Raw val)

    @AssemblyBuilder { self & instructions }

addStoreInstr = \@AssemblyBuilder self, varNum ->
    instructions =
        List.append self.instructions (OpCode Store)
        |> List.append (Raw varNum)

    @AssemblyBuilder { self & instructions }

addLoadInstr = \@AssemblyBuilder self, varNum ->
    instructions =
        List.append self.instructions (OpCode Load)
        |> List.append (Raw varNum)

    @AssemblyBuilder { self & instructions }

addCallInstr = \@AssemblyBuilder self, labelName ->
    instructions =
        List.append self.instructions (OpCode Call)
        |> List.append (Label labelName)

    @AssemblyBuilder { self & instructions }

swapSymbolTable = \@AssemblyBuilder self, localSymbolTable ->
    (@AssemblyBuilder { self & localSymbolTable }, self.localSymbolTable)

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

compileAndTest = \source, pred ->
    ast <- parseStr source |> Debug.okAnd
    asm <- genAssembly ast |> Debug.okAnd

    pred asm


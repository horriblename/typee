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
    staticData : List Instr,
    symbolsInCurrentTable : U64,
    localSymbolTable : Dict Str U64,
    instructions : Assembly,
}

BuildProblem : [
    EmptyFormOrBadFunctionName { body : List Expr },
    WrongArgCount,
    UndeclaredVariable Str,
    VariableRedefined Str,
]

## An intermediate representation of the final byte code, with labels that are to be resolved by the assembler
Assembly : List AsmInstr

dump = \asm ->
    dumpInstr = \instr ->
        when instr is
            OpCode opcode -> "\t$(Inspect.toStr opcode)"
            Raw code -> "\t$(Num.toStr code)"
            Label name -> "\tLABEL $(name)"
            LabelDef name -> "$(name):"

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
        staticData: [],
        localSymbolTable: Dict.empty {},
        instructions: [],
    }

finish : AssemblyBuilder -> Assembly
finish = \@AssemblyBuilder builder ->
    if Dict.contains builder.globalFunctions "main" then
        Dict.values builder.globalFunctions
        |> List.join
        |> \globalFuncs -> List.join [
                [OpCode Call, Label "main", OpCode Halt],
                globalFuncs,
                builder.instructions, # uh, idk what to do with this
            ]
    else
        Dict.values builder.globalFunctions
        |> List.join
        |> \globalFuncs -> List.join [
                builder.instructions,
                [OpCode Halt],
                globalFuncs,
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

        StrLit str ->
            self
            |> extendStaticData (strToWords str)
            |> \(self1, strAddr) -> self1 |> addPushInstr strAddr
            |> Ok

        Record _ -> crash "todo"
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

            # HACK: couldn't be assed to rework stuff so here's a lil hack to
            # swap the assembly and swap it back later
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

        Do exprs ->
            self
            |> genExprs exprs

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
    |> \sym ->
        when sym is
            Err KeyNotFound -> Ok (declareVariable self name)
            Ok _ -> Err (VariableRedefined name)

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

        _ -> Err (EmptyFormOrBadFunctionName { body: formBody })

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

extendStaticData = \@AssemblyBuilder self, data ->
    addr = List.len self.staticData
    staticData = List.join [self.staticData, data]

    (@AssemblyBuilder { self & staticData }, addr)

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

# Turns a list of elements into a list of chunks of the elements.
# Each chunk has the same given size, except the last one, which may be smaller
#
# Example:
#
# ```
# expect chunks [1, 2, 3, 4 , 5, 6, 7, 8] 3 == [[1, 2, 3], [4, 5, 6], [7, 8]]
# ```
chunks : List a, U64 -> List (List a)
chunks = \list, chunkSize ->
    if List.len list == 0 then
        []
    else
        outLen = List.len list |> Num.divCeil chunkSize
        init = List.withCapacity outLen
        List.walk list init \listOfChunks, el ->
            withNewChunk =
                List.withCapacity chunkSize
                |> List.append el
                |> \newChunk -> List.append listOfChunks newChunk

            when List.last listOfChunks is
                Err ListWasEmpty ->
                    withNewChunk

                Ok chunk if List.len chunk >= chunkSize ->
                    withNewChunk

                Ok _ ->
                    List.update
                        listOfChunks
                        (List.len listOfChunks - 1)
                        \chunk1 -> List.append chunk1 el

expect chunks [] 3 == []
expect chunks [1, 2, 3, 4, 5, 6, 7, 8] 3 == [[1, 2, 3], [4, 5, 6], [7, 8]]
expect chunks [1, 2, 3, 4, 5, 6, 7, 8, 9] 3 == [[1, 2, 3], [4, 5, 6], [7, 8, 9]]

u8ToBigEndianWords : List U8 -> List U64
u8ToBigEndianWords = \bytes ->
    # # U64 fits 8 * U8
    bytesPerWord = 8_u8

    bytes
    |> chunks 8
    |> List.map \chunk ->
        List.walkWithIndex chunk 0_u64 \word, byte, i ->
            idx = i |> Num.toU8
            shiftSize = (bytesPerWord - 1 - idx) * 8
            word
            |> Num.bitwiseOr (byte |> Num.toU64 |> Num.shiftLeftBy shiftSize)

expect
    u8ToBigEndianWords [0x01, 0x02, 0x03]
    |> Debug.expectEql [
        Num.shiftLeftBy 0x01 (7 * 8)
        |> Num.bitwiseOr (Num.shiftLeftBy 0x02 (6 * 8))
        |> Num.bitwiseOr (Num.shiftLeftBy 0x03 (5 * 8)),
    ]

strToWords : Str -> List U64
strToWords = \str -> str
    |> Str.toUtf8
    |> u8ToBigEndianWords
    |> List.append 0_u64 # strings are null-terminated (for now)

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


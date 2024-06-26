module [main]

import pf.Arg
import pf.Task exposing [Task, await, attempt]
import pf.File
import pf.Path
import Backend.StackVm.Assembler exposing [compileFromAsciiSource]
import Backend.StackVm.Machine exposing [new, run]

# Arg Parser seems to be broken, I'll hold off complex args for now

Config : {
    infile : Str,
}

parseArgs : Task Config [MissingArgument]
parseArgs =
    args <- await Arg.list
    infile <- List.get args 1
        |> tryOr \_ -> Task.err MissingArgument

    Task.ok { infile }

tryOr = \result, onErr, onOk ->
    when result is
        Ok ok -> onOk ok
        Err err -> onErr err

main : Task {} *
main =
    configRes <- parseArgs |> Task.attempt
    config <- tryOr configRes \err -> crash "Error parsing args: $(Inspect.toStr err)"

    sourceRes <- File.readBytes (Path.fromStr config.infile) |> attempt
    source <- sourceRes
        |> tryOr \err -> crash "Error reading file $(config.infile): $(Inspect.toStr err)"

    program =
        compileFromAsciiSource source
            |> Task.fromResult
            |> Task.onErr! \err -> crash "Compile error: $(Inspect.toStr err)"

    _ <- new program
        |> run
        |> Task.onErr \err -> crash "Runtime error: $(Inspect.toStr err)"
        |> await

    Task.ok {}


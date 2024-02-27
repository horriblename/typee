interface Cli
    exposes [main]
    imports [
        pf.Arg,
        pf.Task.{ Task, await, attempt },
        pf.File,
        pf.Path,
        Parse.{ parse },
        Backend.StackVm.Assembler.{ compileFromAsciiSource },
        Backend.StackVm.Machine.{ new, run },
    ]

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

handleErr : Task ok err, (err -> out), (ok -> out) -> Task out *
handleErr = \task, onErr, onOk ->
    result <- task |> Task.attempt

    when result is
        Ok ok -> onOk ok |> Task.ok
        Err err -> onErr err |> Task.ok

# main : Task {} *
main =
    # config <- parseArgs
    #     |> handleErr \err -> Task.ok {}
    configRes <- parseArgs |> Task.attempt
    config <- tryOr configRes \err -> crash "Error parsing args: \(Inspect.toStr err)"

    dbg config.infile

    sourceRes <- File.readBytes (Path.fromStr config.infile) |> attempt
    source <- sourceRes
        |> tryOr \err -> crash "Error reading file \(config.infile): \(Inspect.toStr err)"

    dbg Str.fromUtf8 source

    program <-
        compileFromAsciiSource source
        |> Task.fromResult
        |> Task.onErr \err -> crash "Compile error: \(Inspect.toStr err)"
        |> await

    _ <- new program
        |> run
        |> Task.onErr \err -> crash "Runtime error: \(Inspect.toStr err)"
        |> await

    Task.ok {}


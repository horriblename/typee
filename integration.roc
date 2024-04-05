app "typeeIntegrationTests"
    packages {
        pf: "https://github.com/roc-lang/basic-cli/releases/download/0.8.1/x8URkvfyi9I0QhmVG98roKBUs_AZRkLFwFJVJ3942YA.tar.br",
    }
    imports [
        pf.Stdout,
        pf.Stderr,
        pf.Task.{ await },
        Debug,
        pf.Cmd.{ Cmd },
    ]
    provides [main] to pf

testFiles = [
    ("basic.eva", "7\n"),
    ("basicFunc.eva", "42\n"),
    ("userFunc.eva", "25\n7\n11\n"),
    ("var.eva", "6\n12\n"),
]

taskLift = \task, onErr, onOk ->
    result <- Task.attempt task
    when result is
        Ok ok -> onOk ok
        Err err -> onErr err

lift = \result, onErr, onOk ->
    when result is
        Ok ok -> onOk ok
        Err err -> onErr err

main : Task.Task {} I32
main =
    compiler = "typee"

    {} <- Stdout.line " ----- Building Compiler -----" |> await

    {} <- Cmd.new "roc"
        |> Cmd.args ["build", "--output", compiler]
        |> Cmd.status
        |> Task.onErr \_ -> Task.ok {} # workaround: roc compiler exits 2 even with just warnings
        |> Task.await
    # |> taskLift \out ->
    #     Stderr.line "Error building compiler: \n\(Inspect.toStr out)"

    {} <- Stdout.line " ----- Built Compiler: ./\(compiler) -----" |> await
    {} <- Stdout.line " ----- Running tests... -----" |> await

    runFile = \file -> Cmd.new "./\(compiler)" |> Cmd.args [file]

    Task.forEach testFiles \(fileName, expectedOut) ->
        out <- runFile "tests/\(fileName)"
            |> Cmd.output
            |> taskLift \(_, err) ->
                Stderr.line "Error encountered, stderr:\n\(Inspect.toStr err)"

        stdoutStr <- Str.fromUtf8 out.stdout
            |> lift \_ -> Stderr.line "Error converting stdout to Str."

        if !(Debug.expectEql stdoutStr expectedOut) then
            {} <- Stderr.line "[\(fileName)] Stdout does not match!" |> await
            Task.err (FailedStdoutCheck)
        else
            Task.ok {}
    |> Task.mapErr \_ -> 1
    |> Task.await \_ ->
        Stdout.line "All tests passed."


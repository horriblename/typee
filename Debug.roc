interface Debug exposes [expectEql, expectFail, what, okAnd]
    imports [pf.Task.{ Task, await }]

expectEql = \left, right ->
    res = left == right

    if !res then
        dbg left

        dbg right

        res
    else
        res

expectFail = \test ->
    if test then
        dbg Expected "expected fail"

        dbg Got test

        !test
    else
        !test

what = \x ->
    dbg x

    x

okAnd = \result, pred ->
    when result is
        Ok ok -> pred ok
        Err err ->
            dbg NotOk err

            Bool.false

succeedAnd : Task ok err, (ok -> Bool) -> Task Bool err where ok implements Inspect, err implements Inspect
succeedAnd = \task, pred ->
    task
    |> Task.mapErr \err ->
        dbg TaskFailed err

        err
    |> await \ok ->
        if pred ok then
            Task.ok Bool.true
        else
            dbg PredicateFaied ok

            Task.ok Bool.false

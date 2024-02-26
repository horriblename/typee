interface Debug exposes [expectEql, what, okAnd]
    imports []

expectEql = \left, right ->
    res = left == right

    if !res then
        dbg left

        dbg right

        res
    else
        res

what = \x ->
    dbg x

    x

okAnd = \result, pred ->
    when result is
        Ok ok -> pred ok
        Err err ->
            dbg NotOk err

            Bool.false


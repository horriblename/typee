interface Debug exposes [expectEql, what]
    imports []

expectEql = \left, right ->
    res = left == right

    if !res then
        dbg Eql left right
        res
    else
        res

what = \x ->
    dbg x
    x

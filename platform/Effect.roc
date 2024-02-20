hosted Effect
    exposes [Effect, after, map, always, forever, putLine, getLine, acquireContext, getVoidType, getIntType, newFunction]
    imports [Context.{ Context }, Type.{ Type }, Function.{ Function }]
    generates Effect with [after, map, always, forever]

putLine : Str -> Effect {}

getLine : Effect Str

acquireContext : Effect Context

getVoidType : Context -> Effect Type

getIntType : Context -> Effect Type

NewFunctionArgs : {
    context : Context,
    returnType : Type,
    exportFlag : Bool,
    name : Str,
    variadic : Bool,
}

newFunction : NewFunctionArgs -> Effect Function


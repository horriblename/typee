platform "effects"
    requires {} { main : Effect.Effect {} }
    exposes []
    packages {}
    imports [Effect Context]
    provides [mainForHost]

mainForHost : Effect.Effect {}
mainForHost = main

Context := {ptr: Nat}
Type := {ptr: Nat}
Function := {ptr: Nat}
Param := {ptr: Nat}

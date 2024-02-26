interface Backend.StackVm.Frame
    exposes [Frame, empty, getVariable, setVariable, returnAddr]
    imports []

Frame := {
    variables : Dict U64 U64,
    returnAddress : U64,
}

empty : U64 -> Frame
empty = \returnAddress -> @Frame { variables: Dict.empty {}, returnAddress }

getVariable = \@Frame frame, varNumber ->
    Dict.get frame.variables varNumber

setVariable = \@Frame frame, varNumber, value ->
    @Frame { frame & variables: Dict.insert frame.variables varNumber value }

returnAddr = \@Frame frame -> frame.returnAddress

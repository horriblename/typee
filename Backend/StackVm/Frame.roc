interface Backend.StackVm.Frame
    exposes [Frame, empty, getVariable, setVariable]
    imports []

Frame := {
    variables : Dict U64 U64,
}

empty : {} -> Frame
empty = \{} -> @Frame { variables: Dict.empty {} }

getVariable = \@Frame frame, varNumber ->
    Dict.get frame.variables varNumber

setVariable = \@Frame frame, varNumber, value ->
    @Frame { frame & variables: Dict.insert frame.variables varNumber value }

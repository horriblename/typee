interface Backend.StackVm.NonEmptyStack
    # FIXME: compiler bug, can't qualify by Module.func, using unique names for now
    exposes [NonEmptyStack, single, pushNES, popNES, last, updateLast]
    imports []

NonEmptyStack t := {
    first : t,
    others : List t,
}

## creates a NonEmptyStack with a single item
single : t -> NonEmptyStack t
single = \first -> @NonEmptyStack { first, others: [] }

pushNES : NonEmptyStack t, t -> NonEmptyStack t
pushNES = \@NonEmptyStack stack, item ->
    @NonEmptyStack { stack & others: List.append stack.others item }

popNES : NonEmptyStack t -> Result (NonEmptyStack t, t) [AttemptToPopLastItem]
popNES = \@NonEmptyStack stack ->
    when stack.others is
        [.. as rest, lastItem] ->
            Ok (
                @NonEmptyStack { stack & others: rest },
                lastItem,
            )

        _ -> Err AttemptToPopLastItem

last : NonEmptyStack t -> t
last = \@NonEmptyStack stack ->
    List.last stack.others
    |> Result.withDefault stack.first

updateLast : NonEmptyStack t, (t -> t) -> NonEmptyStack t
updateLast = \@NonEmptyStack stack, f ->
    lastIdx = List.len stack.others
    if lastIdx > 0 then
        others = List.update stack.others lastIdx f
        @NonEmptyStack { stack & others }
    else
        first = f stack.first
        @NonEmptyStack { stack & first }

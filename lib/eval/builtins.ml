open Scope

let plus args =
   List.fold_left 
      (fun sum obj -> match obj with
         | Int n -> sum + n
         | _ -> raise (Failure "addition with non-int")
      ) 
      0
      args
   |> (fun n -> Int n)
;;

let add_builtins scope_stack =
   Scope.define_local scope_stack "+" (Builtin plus)


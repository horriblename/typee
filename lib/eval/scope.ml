type obj
   = Null
   | Int of int
   | Function of {
      name: string;
      (* args: Ast.expr list; *)
      body: Ast.expr;
   }
   | Builtin of (obj list -> obj)
;;

type scope = (string, obj) Hashtbl.t;;
type scope_stack = scope list;;

let _list_varnames (stack:scope_stack) =
   let _ = print_endline ("registered: " ^
      (Int.to_string (List.fold_left 
         (fun sum scope -> sum + Hashtbl.length scope)
         0
         stack))) in
   let _ = List.map (fun scope -> Hashtbl.iter (fun name _val -> print_endline name) scope) stack
   in ()

let create_scope_stack =
   [Hashtbl.create 10]
;;

let add_scope stack =
   Hashtbl.create 0 :: stack
;;

let pop_scope (stack:scope_stack) =
   match stack with
   | _ :: rest -> rest
   | _ -> raise (Failure "bug: empty scope stack")
;;

let define_local (stack:scope_stack) name value =
   Hashtbl.replace (List.nth stack 0) name value
;;

let find (stack:scope_stack) name =
   List.find_map (fun scope -> Hashtbl.find_opt scope name) stack
;;

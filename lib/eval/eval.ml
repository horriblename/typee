open Scope

let find_var_or_throw scopes var =
   match Scope.find !scopes var with
      | Some v -> v
      | None -> raise (Invalid_argument ("variable '" ^ var ^ "' does not exist"))

let call_obj f args =
   match f with
   | Builtin f' -> f' args
   | Function _ -> raise (Failure "todo")
   | _ -> raise (Failure "todo")

let rec eval_expr (stmt:Ast.expr) (scopes: scope_stack ref) =
   match stmt with
      | Ast.Call (Symbol name, args) ->
         let func = find_var_or_throw scopes name in
         call_obj func (List.map (fun arg -> eval_expr arg scopes) args)
      | Ast.Call (_, _) -> raise (Failure "todo")
      | Ast.Int n -> Int n
      | Ast.Symbol name -> find_var_or_throw scopes name
      | Ast.If (_, _) -> raise (Failure "todo")
      | Ast.Def _ -> raise (Failure "todo")
;;

let% test "eval int" =
   let got = eval_expr (Ast.Int 42) (ref create_scope_stack) in
   match got with
   | Int n -> n == 42
   | _ -> false

let% test "eval add" =
   let scopes = create_scope_stack in
   let () = Builtins.add_builtins scopes in
   let got = eval_expr 
      (Ast.Call ((Ast.Symbol "+"), [Ast.Int 2; Ast.Int 3; Ast.Int 4]))
      (ref scopes) 
   in match got with
      | Int n -> n == 9
      | _ -> false

let rec eval_program stmts (scopes: scope_stack ref) =
   match stmts with
   | stmt :: [] -> eval_expr stmt scopes
   | stmt :: rest ->
         let _ = eval_expr stmt scopes in
         eval_program rest scopes
   | [] -> Null

let run_program stmts =
   let scopes = create_scope_stack in
   let () = Builtins.add_builtins scopes in
   eval_program stmts (ref scopes)
;;


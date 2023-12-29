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

type scope;;
type scope_stack;;

val create_scope_stack : scope_stack;;
val pop_scope : scope_stack -> scope_stack;;
val add_scope : scope_stack -> scope_stack;;
val define_local : scope_stack -> string -> obj -> unit;;
val find : scope_stack -> string -> obj option;;

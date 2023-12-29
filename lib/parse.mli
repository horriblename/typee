type parse_err
   = ExpectedEOF
;;

val parse : string -> (Ast.expr list, parse_err) result;;

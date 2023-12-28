type parse_err
   = ExpectedEOF
;;

val parse : string -> (Ast.node list, parse_err) result;;

open Opal

let parse_symbol = many1 alpha_num;;

let parse_expr source = parse_symbol source;;

(* let parse_call source = *)
(*    let between (exactly '(') (exactly ')') in *)

let parse = many1 parse_expr;;

      


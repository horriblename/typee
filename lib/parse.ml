open Opal
open Ast

(* ChatGPT said I should put type defs in .ml and .mli; not sure if I believe it but whatevs *)
type parse_err
   = ExpectedEOF
;;

let parse_symbol = many1 alpha_num;;

let rec parse_call source = 
   let args_parser = sep_by1 (parse_expr) (skip_many1 space) in
   let parser = between (exactly '(') (exactly ')') args_parser
      => (fun args -> match args with
         | func :: rest -> Call (func, rest)
         | _ -> raise (Failure "unreachable"))
   in parser source

and parse_expr source =
   let parser = (parse_symbol => (fun sym -> Symbol (implode sym)))
      <|> parse_call
   in parser source
;;

(* let parse_call source = *)
(*    let between (exactly '(') (exactly ')') in *)

let parse source =
   source
      |> LazyStream.of_string
      |> many1 parse_expr
      |> Option.map (fun (syntax_tree, _rest) -> syntax_tree)
      |> Option.to_result ~none:ExpectedEOF
;;


let%test "parse a symbol" =
   let got = parse "foo" in
   match got with
   | Ok [Symbol "foo"] -> true
   | _ -> false
;;

let%test "parse a function call" =
   let got = parse "(foo bar)" in
   match got with
   | Ok [Call (Symbol "foo", [Symbol "bar"])] -> true
   | _ -> false

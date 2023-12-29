open Opal
open Ast

(* ChatGPT said I should put type defs in .ml and .mli; not sure if I believe it but whatevs *)
type parse_err
   = ExpectedEOF
;;

let tupled (p1: ('t, 'r) parser) (p2: ('t, 's) parser) input =
   match p1 input with
   | Some (r1, input) -> (p2 => fun r2 -> (r1, r2)) input
   | None -> None

let symbol_string = many1 alpha_num => implode;;
let parse_symbol = many1 alpha_num => (fun sym -> Symbol (implode sym));;

let parenthesized = between (exactly '(') (exactly ')') ;;
(* FIXME: I don't understand generics, so I just duplicate parenthesized *)
let parenthesized' = between (exactly '(') (exactly ')') ;;

let rec parse_call source = 
   let args_parser = sep_by1 (parse_expr) (skip_many1 space) in
   let parser = parenthesized args_parser
      => (fun args -> match args with
         | func :: rest -> Call (func, rest)
         | _ -> raise (Failure "unreachable"))
   in parser source

and parse_def source =
   let args_parser = parenthesized' (many symbol_string) in
   let content_parser =
      (* couldn't get let* to work :c *)
      token "def" >> 
      tupled 
         symbol_string 
         (tupled 
            args_parser
            parse_expr
         )
      => fun (name, (args, body)) -> Def {
         name = name;
         args = args;
         body = body;
      } in
   content_parser source

and parse_expr source =
   let parser = parse_symbol <|> parse_def <|> parse_call
   in parser source
;;

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


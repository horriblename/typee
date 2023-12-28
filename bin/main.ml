open Typee

let () = 
   match Parse.parse "hello" with
      | Ok [Symbol sym] -> print_endline sym
      | _ -> print_endline "nothing"
;;

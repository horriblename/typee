type argument = string;;

type expr
   = Call of expr * expr list
   | Int of int
   | Symbol of string
   | If of expr * expr
   | Def of {
      name: string;
      args: argument list;
      body: expr;
   }
;;


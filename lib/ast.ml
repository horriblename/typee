open! Core

type argument = string [@@deriving sexp];;

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
   | Assign of {
      name: string;
      value: expr;
   }
   [@@deriving sexp]
;;


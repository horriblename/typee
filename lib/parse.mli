type node 
   = Call of node list
   | Symbol of string;;

val parse : string -> node;;

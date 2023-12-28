
type node 
   = Call of node * node list
   | Symbol of string
;;


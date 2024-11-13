package simplesub

import "github.com/horriblename/typee/src/internal/scope"

var intBinaryOptType = Func{
	Args: []SimpleType{Int{}, Int{}},
	Ret:  Int{},
}

var intComparatorType = Func{
	Args: []SimpleType{Int{}, Int{}},
	Ret:  Bool{},
}

func addBuiltins(scope *scope.ScopedMap[TypeScheme]) {
	scope.Insert("+", intBinaryOptType)
	scope.Insert("-", intBinaryOptType)
	scope.Insert("*", intBinaryOptType)
	scope.Insert("/", intBinaryOptType)
	scope.Insert(">", intComparatorType)
	scope.Insert("<", intComparatorType)
	scope.Insert("=", intComparatorType)
	scope.Insert("print", Func{
		Args: []SimpleType{Str{}},
		Ret:  Str{},
	})
}

func addBuiltinTypes(types *scope.ScopedMap[TypeScheme]) {
	types.Insert("Int", Int{})
	types.Insert("Str", Str{})
	types.Insert("Bool", Bool{})
}

package simplesub

import "github.com/horriblename/typee/src/internal/scope"

var intBinaryOptType = Func{
	Args: []SimpleType{Int{}, Int{}},
	Ret:  Int{},
}

var intComparatorType = Func{
	Args: []SimpleType{Int{}, Int{}},
	Ret:  Primitive{PrimitiveBool},
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
	scope.Insert("exit", Func{
		Args: []SimpleType{Int{}},
		Ret:  Bot{},
	})
}

func addBuiltinTypes(types *scope.ScopedMap[TypeScheme]) {
	types.Insert("Int", Int{})
	types.Insert("U8", Int{})
	types.Insert("U16", Int{})
	types.Insert("U32", Int{})
	types.Insert("U64", Int{})
	types.Insert("I8", Int{})
	types.Insert("I16", Int{})
	types.Insert("I32", Int{})
	types.Insert("I64", Int{})
	types.Insert("Str", Str{})
	types.Insert("Bool", Primitive{PrimitiveBool})
	types.Insert("Opaque", Primitive{PrimitiveOpaque})
}

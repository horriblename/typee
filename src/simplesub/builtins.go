package simplesub

import (
	"github.com/horriblename/typee/src/internal/scope"
)

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
		Ret:  Record{},
	})
	scope.Insert("exit", Func{
		Args: []SimpleType{Int{}},
		Ret:  Bot{},
	})
	t := freshVar()
	scope.Insert("at", PolymorphicType{
		Body: Func{
			Args: []SimpleType{
				SliceType{t},
				Int{},
			},
			Ret: t,
		},
	})
	scope.Insert("strFromCStr", PolymorphicType{
		Body: Func{
			Args: []SimpleType{Primitive{PrimitiveOpaque}},
			Ret:  Str{},
		},
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
	types.Insert("Float", Primitive{PrimitiveFloat})
	types.Insert("F64", Primitive{PrimitiveFloat})
	types.Insert("F32", Primitive{PrimitiveFloat})
	types.Insert("Bool", Primitive{PrimitiveBool})
	types.Insert("Opaque", Primitive{PrimitiveOpaque})
}

package simplesub

import (
	"github.com/horriblename/typee/src/internal/scope"
	"github.com/horriblename/typee/src/opt"
)

var I64 = Int{true, 64}

var intBinaryOptType = Func{
	Args: []SimpleType{I64, I64},
	Ret:  I64,
}

var intComparatorType = Func{
	Args: []SimpleType{I64, I64},
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
		Args: []SimpleType{I64},
		Ret:  Bot{},
	})
	t := freshVar()
	scope.Insert("at", PolymorphicType{
		Body: Func{
			Args: []SimpleType{
				SliceType{t},
				I64,
			},
			Ret: t,
		},
	})

	scope.Insert("null", PolymorphicType{
		Body: Ref{freshVar()},
	})

	t = freshVar()
	scope.Insert("stackAlloc", PolymorphicType{
		Body: Func{
			Args: []SimpleType{},
			Ret:  Ref{t},
		},
	})

	t = freshVar()
	scope.Insert("deref", PolymorphicType{
		Body: Func{
			Args: []SimpleType{Ref{t}},
			Ret:  t,
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
	types.Insert("Int", I64)
	types.Insert("U8", Int{false, 8})
	types.Insert("U16", Int{false, 16})
	types.Insert("U32", Int{false, 32})
	types.Insert("U64", Int{false, 64})
	types.Insert("I8", Int{true, 8})
	types.Insert("I16", Int{true, 16})
	types.Insert("I32", Int{true, 32})
	types.Insert("I64", Int{true, 64})
	types.Insert("Str", Str{})
	types.Insert("Float", Primitive{PrimitiveFloat})
	types.Insert("F64", Primitive{PrimitiveFloat})
	types.Insert("F32", Primitive{PrimitiveFloat})
	types.Insert("Bool", Primitive{PrimitiveBool})
	types.Insert("Opaque", Primitive{PrimitiveOpaque})

	a := freshVar()
	types.Insert("Ref", PolymorphicType{
		Body: Ref{
			Content: a,
		},
		TypeParams: opt.Some([]uint{a.Uid()}),
	})
}

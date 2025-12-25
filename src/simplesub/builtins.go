package simplesub

import (
	"github.com/horriblename/typee/src/can"
	"github.com/horriblename/typee/src/opt"
	"github.com/horriblename/typee/src/parse"
)

var I64 = Int{true, 64}

var intBinaryOptType = Func{Args: []SimpleType{I64, I64}, Ret: I64}

var intComparatorType = Func{Args: []SimpleType{I64, I64}, Ret: Primitive{PrimitiveBool}}

var _builtinVars map[string]TypeScheme

func builtinVars() map[string]TypeScheme {
	if _builtinVars != nil {
		return _builtinVars
	}
	at_t := freshVar()
	deref_t := freshVar()
	listForEach_t := freshVar()

	_builtinVars = map[string]TypeScheme{
		"+":     intBinaryOptType,
		"-":     intBinaryOptType,
		"*":     intBinaryOptType,
		"/":     intBinaryOptType,
		">":     intComparatorType,
		"<":     intComparatorType,
		"=":     intComparatorType,
		"print": Func{Args: []SimpleType{Str{}}, Ret: Record{}},
		"exit":  Func{Args: []SimpleType{I64}, Ret: Bot{}},
		"at": PolymorphicType{
			Body: Func{Args: []SimpleType{
				SliceType{at_t},
				I64,
			}, Ret: at_t},
		},
		"null": PolymorphicType{
			Body: Ref{freshVar()},
		},

		"stackAlloc": PolymorphicType{
			Body: Func{Args: []SimpleType{}, Ret: Ref{freshVar()}},
		},

		"deref": PolymorphicType{
			Body: Func{Args: []SimpleType{Ref{deref_t}}, Ret: deref_t},
		},

		"strFromCStr": PolymorphicType{
			Body: Func{Args: []SimpleType{Primitive{PrimitiveOpaque}}, Ret: Str{}},
		},
		"strToCStr": PolymorphicType{
			Body: Func{Args: []SimpleType{Str{}}, Ret: Primitive{PrimitiveOpaque}},
		},
		"typeOf": PolymorphicType{
			Body: Func{Args: []SimpleType{Primitive{PrimitiveOpaque}}, Ret: Primitive{PrimitiveOpaque}},
		},
		"i64ToI32": PolymorphicType{
			Body: Func{Args: []SimpleType{I64}, Ret: Int{true, 32}},
		},
		"i64ToI8": PolymorphicType{
			Body: Func{Args: []SimpleType{I64}, Ret: Int{true, 8}},
		},
		"emptyList": PolymorphicType{
			Body: Func{
				Args:   []SimpleType{},
				Ret:    SliceType{freshVar()},
				Method: false,
			},
		},
		"forEach": PolymorphicType{
			Body: Func{
				Args: []SimpleType{
					SliceType{listForEach_t},
					Func{
						Args: []SimpleType{Ref{listForEach_t}},
						Ret:  Record{},
					},
				},
				Ret:    Record{},
				Method: false,
			},
		},
		"ptrToI64": PolymorphicType{
			Body: Func{Args: []SimpleType{Primitive{PrimitiveOpaque}}, Ret: I64},
		},
	}
	return _builtinVars
}

var _builtinTypes map[string]TypeScheme

var StdModName can.ModuleName = "Std"
var gobject = ObjectType{
	Module:  StdModName,
	Kind:    parse.Class,
	Name:    "Object",
	Supers:  []Application{},
	Fields:  []NamedMember{},
	Methods: []NamedMember{},
	Top:     true,
}

func builtinTypes() map[string]TypeScheme {
	if _builtinTypes != nil {
		return _builtinTypes
	}

	ref_t := freshVar()

	_builtinTypes = map[string]TypeScheme{
		"Int":    I64,
		"U8":     Int{false, 8},
		"U16":    Int{false, 16},
		"U32":    Int{false, 32},
		"U64":    Int{false, 64},
		"I8":     Int{true, 8},
		"I16":    Int{true, 16},
		"I32":    Int{true, 32},
		"I64":    Int{true, 64},
		"Str":    Str{},
		"Float":  Primitive{PrimitiveFloat},
		"F64":    Primitive{PrimitiveFloat},
		"F32":    Primitive{PrimitiveFloat},
		"Bool":   Primitive{PrimitiveBool},
		"Opaque": Primitive{PrimitiveOpaque},
		"GType":  Primitive{PrimitiveOpaque}, // this should be a guintptr

		"Ref": PolymorphicType{
			Body: Ref{
				Content: ref_t,
			},
			TypeParams: opt.Some([]uint{ref_t.Uid()}),
		},
		"Object": gobject,
		"CClosure": Record{
			Fields: []NamedType{
				{Name: "func", Type: Primitive{PrimitiveOpaque}},
				{Name: "data", Type: Primitive{PrimitiveOpaque}},
				{Name: "cleanup", Type: Primitive{PrimitiveOpaque}},
			},
		},
	}
	return _builtinTypes
}

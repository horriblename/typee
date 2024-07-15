package solve

import (
	"github.com/horriblename/typee/src/types"
)

var intBinaryOpFuncType = types.Func{
	Args: []types.Type{&types.Int{}},
	Ret: &types.Func{
		Args: []types.Type{&types.Int{}},
		Ret:  &types.Int{},
	},
}

var intComparatorOpFuncType = types.Func{
	Args: []types.Type{&types.Int{}},
	Ret: &types.Func{
		Args: []types.Type{&types.Int{}},
		Ret:  &types.Bool{},
	},
}

var builtins = map[string]types.Type{
	"+": &intBinaryOpFuncType,
	"-": &intBinaryOpFuncType,
	"*": &intBinaryOpFuncType,
	"/": &intBinaryOpFuncType,
	">": &intComparatorOpFuncType,
	"<": &intComparatorOpFuncType,
	"=": &intComparatorOpFuncType,
}

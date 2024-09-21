package solve

import (
	"github.com/horriblename/typee/src/types"
)

var intBinaryOpFuncType = types.Func{
	Args: []types.Type{&types.Int{}, &types.Int{}},
	Ret:  &types.Int{},
}

var intComparatorOpFuncType = types.Func{
	Args: []types.Type{&types.Int{}, &types.Int{}},
	Ret:  &types.Bool{},
}

var builtins = map[string]types.Type{
	"+": &intBinaryOpFuncType,
	"-": &intBinaryOpFuncType,
	"*": &intBinaryOpFuncType,
	"/": &intBinaryOpFuncType,
	">": &intComparatorOpFuncType,
	"<": &intComparatorOpFuncType,
	"=": &intComparatorOpFuncType,
	"print": &types.Func{
		Args: []types.Type{&types.String{}},
		// TODO: switch to unit type when they're are impl'd
		Ret: &types.String{},
	},
}

package solve

import (
	"github.com/horriblename/typee/src/opt"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/types"
)

func builtinsType(name string) opt.Option[types.Type] {
	intBinaryOpFuncType := types.Func{
		Args: []types.Type{&types.Int{}, &types.Int{}},
		Ret:  &types.Int{},
	}
	switch name {
	case "+":
		return opt.Some(types.Type(&intBinaryOpFuncType))
	case "-":
		return opt.Some(types.Type(&intBinaryOpFuncType))
	case "*":
		return opt.Some(types.Type(&intBinaryOpFuncType))
	case "/":
		return opt.Some(types.Type(&intBinaryOpFuncType))
	default:
		return opt.None[types.Type]()
	}
}

func literalType(expr parse.Expr) opt.Option[types.Type] {
	switch expr.(type) {
	case *parse.BoolLiteral:
		return opt.Some(types.Type(&types.Bool{}))
	case *parse.IntLiteral:
		return opt.Some(types.Type(&types.Int{}))
	case *parse.StrLiteral:
		return opt.Some(types.Type(&types.String{}))
	default:
		return opt.None[types.Type]()
	}
}

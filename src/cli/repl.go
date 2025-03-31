package main

import (
	"strings"

	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/simplesub"
	"github.com/horriblename/typee/src/types"
)

type replState struct {
	typer   *simplesub.Typer
	rawType bool
}

func processInput(ctx replState, line string) {
	line = strings.TrimSpace(line)

	expr, err := parse.ParseString(line)
	if err != nil {
		errorf("%s", err.Error())
		return
	}

	if len(expr) == 0 {
		return
	}

	ty, _, err := ctx.typer.TypeProgram(expr)
	if err != nil {
		errorf("%s", err)
		return
	}

	if ctx.rawType {
		errorf("pre-simplify: (polymorphic) %s", ty[0].String())
	}

	st, ok := ty[0].(simplesub.SimpleType)
	if !ok {
		st = ty[0].(simplesub.PolymorphicType).Body
	}
	simplified := simplesub.SimplifyType(st)

	if ctx.rawType {
		errorf("pre-coalesce: (polymorphic) %v", simplified)
	}

	simpleTy := simplesub.CoalesceType(simplified)

	errorf(": %s", types.DeepPrint(simpleTy))
	return
}

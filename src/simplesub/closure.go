package simplesub

import (
	"maps"
	"slices"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/internal/scope"
	"github.com/horriblename/typee/src/parse"
)

type findClosureCtx struct {
	// list of locals used
	vars scope.ScopedMap[int]

	// store the set of all variables that were accessed out-of-scope for each fn
	outOfScopeAccesses []outsideAccesses

	innerMostFnLevel int

	Captures map[int][]string
}

type outsideAccesses struct {
	lvl      int
	accesses map[string]unit
}

func captureClosures(ctx *findClosureCtx, expr parse.Expr) {
	// TODO: variable shadowing
	// or I just take care of it via canonicalization?

	switch e := expr.(type) {
	case *parse.ArrayLiteral:
		for _, subexpr := range e.Elements {
			captureClosures(ctx, subexpr)
		}
	case *parse.CaseExpr:
		panic("TODO finding closure in case expr")
	case *parse.ExternCall:
		for _, subexpr := range e.Args {
			captureClosures(ctx, subexpr)
		}
	case *parse.Fn:
		prevLvl := ctx.innerMostFnLevel
		ctx.vars.NewScope()
		ctx.innerMostFnLevel = ctx.vars.ScopeLevel()
		ctx.outOfScopeAccesses = append(ctx.outOfScopeAccesses, outsideAccesses{
			lvl:      ctx.vars.ScopeLevel(),
			accesses: map[string]unit{},
		})

		for _, arg := range e.Args {
			ctx.vars.Insert(arg, ctx.vars.ScopeLevel())
		}

		captureClosures(ctx, e.Body)

		captures, ok := popSlice(&ctx.outOfScopeAccesses).Unwrap()
		assert.True(ok, "BUG corrupted outOfScopeAccesses")
		ctx.Captures[e.ID()] = fun.Collect(maps.Keys(captures.accesses))
		ctx.innerMostFnLevel = prevLvl
		ctx.vars.PopScope()
	case *parse.Form:
		for _, subexpr := range e.Children {
			captureClosures(ctx, subexpr)
		}
	case *parse.FuncDef:
		ctx.vars.NewScope()
		for _, arg := range e.Args {
			ctx.vars.Insert(arg, ctx.vars.ScopeLevel())
		}
		for _, subexpr := range e.Body {
			captureClosures(ctx, subexpr)
		}
		ctx.vars.PopScope()
	case *parse.IfExpr:
		captureClosures(ctx, e.Condition)
		captureClosures(ctx, e.Consequence)
		captureClosures(ctx, e.Alternative)
	case *parse.LetExpr:
		ctx.vars.NewScope()
		for _, ass := range e.Assignments {
			ctx.vars.Insert(ass.Var, ctx.vars.ScopeLevel())
			captureClosures(ctx, ass.Value)
		}
		ctx.vars.PopScope()
	case *parse.MethodAccess:
		captureClosures(ctx, e.Obj)
	case *parse.Record:
		for _, field := range e.Fields {
			captureClosures(ctx, field.Value)
		}
	case *parse.RecordAccess:
		captureClosures(ctx, e.Record)
	case *parse.Set:
		ctx.checkOutOfScopeAccess(e.Name)
		captureClosures(ctx, e.Value)
	case *parse.Symbol:
		// TODO: should I check this is actually a variable?
		ctx.checkOutOfScopeAccess(e.Name)
	case *parse.TaggedExpr:
		panic("TODO tagged expression capture analysis")
	case *parse.VarDef: // ok did I ever implement var
		ctx.vars.Insert(e.Name, ctx.vars.ScopeLevel())
	default:
	}
}

func (self *findClosureCtx) checkOutOfScopeAccess(name string) {
	if lvl, ok := self.vars.Get(name).Unwrap(); ok {
		if lvl >= self.innerMostFnLevel {
			// variable is declared within the scope of the innermost
			// closure, not a captured variable
			return
		}

		// TODO: terrible impl, I should read up on the OCaml levels thing again
		for _, acc := range slices.Backward(self.outOfScopeAccesses) {
			if acc.lvl <= lvl {
				break
			}
			acc.accesses[name] = unit{}
		}

		// ctx.accessed[]
	}
}

package simplesub

import (
	orderedset "github.com/horriblename/typee/src/internal/ordered_set"
	"github.com/horriblename/typee/src/internal/scope"
	"github.com/horriblename/typee/src/parse"
	"github.com/looplab/tarjan"
)

// Perform dependency analysis between top level symbols and
// find groups of mutually recursive functions
func (self *Typer) findRecursiveGroups(ast *[]parse.Expr) {

}

type locality bool

const (
	topLevel   = false
	localLevel = true
)

type depCtx struct {
	vars          scope.ScopedMap[locality]
	toplevelIndex map[string]int
}

// returns [][]string but library is from pre-generic era so it's a [][]any :(
// also returns a set of selfRecursive symbols because tarjan lib doesn't
// differentiate between self-recursive and no-recursion
func groupRecursives(ast []parse.Expr) (groups [][]any, selfRecursive map[string]struct{}) {
	ctx := depCtx{
		scope.NewScopedMap[locality](),
		map[string]int{},
	}

	// build dependency graph

	depGraph := map[any][]any{}
	selfRecursive = map[string]struct{}{}

	for i, node := range ast {
		switch n := node.(type) {
		case *parse.FuncDef:
			ctx.vars.Insert(n.Name, topLevel)
			ctx.toplevelIndex[n.Name] = i
		case *parse.Set:
			ctx.vars.Insert(n.Name, topLevel)
			ctx.toplevelIndex[n.Name] = i
		default:
		}
	}

	for i, node := range ast {
		switch n := node.(type) {
		case *parse.FuncDef:
			deps := orderedset.NewOrderedSet[any]()
			ctx.findDependencies(deps, []parse.Expr{n})

			depGraph[n.Name] = deps.Slice()
			if deps.Len() != 0 && deps.Has(n.Name) {
				selfRecursive[n.Name] = struct{}{}
			}

		case *parse.Set:
			deps := orderedset.NewOrderedSet[any]()
			ctx.toplevelIndex[n.Name] = i

			depGraph[n.Name] = deps.Slice()
			if deps.Len() != 0 && deps.Has(n.Name) {
				selfRecursive[n.Name] = struct{}{}
			}

		default:
		}
	}

	return tarjan.Connections(depGraph), selfRecursive
}

func (ctx *depCtx) findDependencies(deps *orderedset.OrderedSet[any], nodes []parse.Expr) {
	for _, node := range nodes {
		switch n := node.(type) {
		case *parse.Fn:
			ctx.vars.NewScope()
			for _, arg := range n.Args {
				ctx.vars.Insert(arg, localLevel)
			}
			groupRecursives(n.ChildNodes())
			ctx.vars.PopScope()
		case *parse.Symbol:
			if loc, ok := ctx.vars.Get(n.Name).Unwrap(); ok && loc == topLevel {
				deps.Insert(n.Name)
			}
		case *parse.FuncDef:
			ctx.vars.NewScope()
			for _, arg := range n.Args {
				ctx.vars.Insert(arg, localLevel)
			}
			ctx.findDependencies(deps, n.ChildNodes())
			ctx.vars.PopScope()

		case *parse.VarDef:
			ctx.vars.Insert(n.Name, localLevel)

		case *parse.LetExpr:
			ctx.vars.NewScope()
			for _, ass := range n.Assignments {
				ctx.vars.Insert(ass.Var, localLevel)
			}
			ctx.findDependencies(deps, n.ChildNodes())
			ctx.vars.PopScope()

		default:
			ctx.findDependencies(deps, node.ChildNodes())
		}
	}
}

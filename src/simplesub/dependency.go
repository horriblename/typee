package simplesub

import (
	"fmt"
	"maps"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/graph"
	"github.com/horriblename/typee/src/graph/tarjan"
	orderedset "github.com/horriblename/typee/src/internal/ordered_set"
	"github.com/horriblename/typee/src/internal/scope"
	"github.com/horriblename/typee/src/parse"
)

type locality bool

const (
	topLevel   = false
	localLevel = true
)

type depCtx struct {
	vars          scope.ScopedMap[locality]
	toplevelIndex map[string]int
}

// returns groups and a set of selfRecursive symbols because tarjan lib doesn't
// differentiate between self-recursive and no-recursion
func groupRecursives(ast []parse.Expr) (groups [][]string, selfRecursive map[string]struct{}) {
	ctx := depCtx{
		scope.NewScopedMap[locality](),
		map[string]int{},
	}

	// build dependency graph

	depGraph := map[string][]string{}
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
			deps := orderedset.NewOrderedSet[string]()
			ctx.findDependencies(deps, []parse.Expr{n})

			depGraph[n.Name] = deps.Slice()
			if deps.Len() != 0 && deps.Has(n.Name) {
				selfRecursive[n.Name] = struct{}{}
			}

		case *parse.Set:
			deps := orderedset.NewOrderedSet[string]()
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

func sortTypeDefs(ast []parse.Expr) (order []string, astLookup map[string]parse.Expr, err error) {
	allDeps := map[string]map[string]unit{}
	typeDefAst := map[string]parse.Expr{}

	for _, node := range ast {
		switch n := node.(type) {
		case *parse.EnumDef:
			typeDefAst[n.Name] = n
			allDeps[n.Name] = map[string]unit{}
		case *parse.UnionDef:
			typeDefAst[n.Name] = n
			allDeps[n.Name] = map[string]unit{}

		case *parse.ObjectTypeDef:
			typeDefAst[n.Name] = n
			deps := map[string]unit{}
			for _, super := range n.Supers {
				deps[super] = unit{}
			}
			for _, member := range n.Fields {
				switch m := member.(type) {
				case parse.ClassField:
					markTypeDeps(m.Type, deps, n.Name)
				case parse.ClassMethod:
					sig, ok := m.Func.Signature.Unwrap()
					if !ok {
						panic(fmt.Errorf("in %s.%s: class method signature is required", n.Name, m.Name()))
					}
					for _, t := range sig {
						markTypeDeps(t, deps, n.Name)
					}
				}
			}
			allDeps[n.Name] = deps

		case *parse.TypeAlias:
			typeDefAst[n.Name] = n
			deps := map[string]unit{}
			markTypeDeps(n.Type, deps, "")
			allDeps[n.Name] = deps
		}
	}

	adjacencyList := fun.MapMap(allDeps, func(d map[string]unit) []string {
		return fun.Collect(maps.Keys(d))
	})

	s, err := graph.Toposort(adjacencyList)
	if err != nil {
		return nil, nil, fmt.Errorf("sorting type definitions: %w", err)
	}

	return s, typeDefAst, nil
}

func markTypeDeps(x parse.TypeRepr, deps map[string]unit, skip string) {
	work := []parse.TypeRepr{x}

	for len(work) != 0 {
		node, ok := popSlice(&work).Unwrap()
		assert.True(ok, "len already checked")

		if n, ok := node.(parse.TypeName); ok && n.Name != skip {
			deps[n.Name] = unit{}
		}

		work = append(work, parse.ChildNodes(node)...)
	}
}

func (ctx *depCtx) findDependencies(deps *orderedset.OrderedSet[string], nodes []parse.Expr) {
	for _, node := range nodes {
		switch n := node.(type) {
		case *parse.Fn:
			ctx.vars.NewScope()
			for _, arg := range n.Args {
				ctx.vars.Insert(arg, localLevel)
			}
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

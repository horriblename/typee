package simplesub

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/horriblename/typee/src/fun"
	orderedset "github.com/horriblename/typee/src/internal/ordered_set"
	"github.com/horriblename/typee/src/parse"
)

type moduleExports struct {
	Types   map[string]TypeScheme
	Globals map[string]TypeScheme
}

type depModule struct {
	name string
	ast  []parse.Expr
	deps []string
}

var (
	ErrCyclicImport = errors.New("cyclic import detected")
)

func (self *Typer) typeDeps(mainModule string, program []parse.Expr) error {
	modules, err := self.sortDeps(mainModule, program)
	if err != nil {
		return err
	}

	for _, mod := range modules {
		_, typTable, err := self.TypeProgram(mod.ast)
		if err != nil {
			return err
		}

		symbols, err := typeTableToSymbolMap(mod.ast, typTable)
		if err != nil {
			return fmt.Errorf("generating symbol table from type table: %w", err)
		}

		self.importedTypes[mod.name] = symbols
	}

	return nil
}

// sort modules in dependency tree, leaf modules (no dependencies) go first
func (self *Typer) sortDeps(module string, program []parse.Expr) ([]depModule, error) {
	depMap, err := self.moduleDeps(module, program)
	if err != nil {
		return nil, err
	}

	sorted, err := toposort(depMap, func(d depModule) []string { return d.deps })
	if err != nil {
		if e, ok := err.(*errCycle[string]); ok {
			return nil, fmt.Errorf("%w: in %s", ErrCyclicImport, e.lastVisited)
		}
		return nil, err
	}

	return fun.Map(sorted, func(mod string) depModule { return depMap[mod] }), nil
}

func (self *Typer) moduleDeps(module string, program []parse.Expr) (map[string]depModule, error) {
	// TODO: rename
	imports := map[string]depModule{}

	parsed := map[string][]parse.Expr{}
	imps := self.getImports(program)
	imports[module] = depModule{
		name: module,
		ast:  program,
		deps: imps,
	}
	q := slices.Clone(imports[module].deps)

	for len(q) != 0 {
		mod, ok := popSlice(&q).Unwrap()
		if !ok {
			panic("unreachable: len(q) checked beforehand")
		}

		if _, ok := imports[mod]; ok {
			continue
		}

		program, ok := parsed[mod]
		if !ok {
			var err error
			program, err = parseModule(mod)
			if err != nil {
				return nil, err
			}
			parsed[mod] = program
		}

		imps := self.getImports(program)
		imports[module] = depModule{
			name: mod,
			ast:  program,
			deps: imps,
		}
		q = append(q, imports[module].deps...)
	}

	return imports, nil
}

func (self *Typer) getImports(program []parse.Expr) []string {
	imports := []string{}
	for _, expr := range program {
		imp, ok := expr.(*parse.Import)
		if !ok {
			return imports
		}

		imports = append(imports, strings.Join(imp.Module, "."))
	}

	return imports
}

func parseModule(name string) ([]parse.Expr, error) {
	modPath := path.Join(strings.Split(name, ".")...) + ".hor"
	data, err := os.ReadFile(modPath)
	if err != nil {
		return nil, fmt.Errorf("error parsing %s: %w", modPath, err)
	}

	return parse.ParseString(string(data))
}

func (self *Typer) TypeModule(module string) {

}

type errCycle[T any] struct {
	lastVisited T
}

func (e *errCycle[T]) Error() string {
	return fmt.Sprintf("cycle detected in toposort, last visited node: %v", e.lastVisited)
}

// sorts a map[nodes]outgoingNodes
// leaf nodes go first, root goes last
func toposort[Node comparable, Data any](edges map[Node]Data, outgoing func(Data) []Node) (_ []Node, err error) {
	defer func() {
		if e := recover(); e != nil {
			if ce, ok := e.(errCycle[Node]); ok {
				err = &ce
			} else {
				panic(e)
			}
		}
	}()

	sorted := []Node{}
	unvisited := orderedset.NewOrderedSet(fun.Collect(maps.Keys(edges))...)
	// false mark is a temporary mark, true is permanent
	mark := map[Node]bool{}

	var visit func(Node)
	visit = func(node Node) {
		unvisited.SwapDelete(node)
		perm, ok := mark[node]
		if ok {
			if perm {
				return
			} else {
				panic(errCycle[Node]{node})
			}
		}

		mark[node] = false

		for _, m := range outgoing(edges[node]) {
			visit(m)
			mark[node] = true
		}

		sorted = append(sorted, node)
	}

	for {
		if node, ok := unvisited.Pop(); ok {
			visit(node)
		} else {
			break
		}
	}

	return sorted, nil
}

func typeTableToSymbolMap(program []parse.Expr, typTable map[int]TypeScheme) (moduleExports, error) {
	symbols := moduleExports{
		Types:   map[string]TypeScheme{},
		Globals: map[string]TypeScheme{},
	}
	for _, expr := range program {
		switch e := expr.(type) {
		case *parse.FuncDef:
			symbols.Globals[e.Name] = typTable[e.ID()]

		case *parse.Set:
			symbols.Globals[e.Name] = typTable[e.ID()]

		case *parse.ObjectTypeDef:
			symbols.Types[e.Name] = typTable[e.ID()]

		case *parse.UnionDef:
			symbols.Types[e.Name] = typTable[e.ID()]

		case *parse.EnumDef:
			symbols.Types[e.Name] = typTable[e.ID()]

		case *parse.TypeAlias:
			symbols.Types[e.Name] = typTable[e.ID()]

		case *parse.Import:
		default:
			return moduleExports{}, fmt.Errorf("%w:\n    %s", ErrInvalidTopLevel, expr.Pretty())
		}
	}

	return symbols, nil
}

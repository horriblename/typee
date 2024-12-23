package simplesub

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

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

func (self *Typer) typeDeps(program []parse.Expr) error {
	for _, mod := range program {
		dep, ok := mod.(*parse.Import)
		if !ok {
			continue
		}

		name := strings.Join(dep.Module, ".")

		if _, ok := self.moduleCache[name]; ok {
			return nil
		}

		ast, err := parseModule(name)
		if err != nil {
			return err
		}

		_, typTable, err := self.TypeProgram(ast)
		if err != nil {
			return err
		}

		symbols, err := typeTableToSymbolMap(ast, typTable)
		if err != nil {
			return fmt.Errorf("generating symbol table from type table: %w", err)
		}

		self.moduleCache[name] = symbols
	}

	return nil
}

// sort modules in dependency tree, leaf modules (no dependencies) go first

func parseModule(name string) ([]parse.Expr, error) {
	modPath := path.Join(strings.Split(name, ".")...) + ".hor"
	data, err := os.ReadFile(modPath)
	if err != nil {
		return nil, fmt.Errorf("error parsing %s: %w", modPath, err)
	}

	return parse.ParseString(string(data))
}

// sorts a map[nodes]outgoingNodes
// leaf nodes go first, root goes last

func typeTableToSymbolMap(program []parse.Expr, typTable map[int]TypeScheme) (moduleExports, error) {
	symbols := moduleExports{
		Types:   map[string]TypeScheme{},
		Globals: map[string]TypeScheme{},
	}
	for _, expr := range program {
		switch e := expr.(type) {
		case *parse.FuncDef:
			symbols.Globals[e.Name] = assertType(typTable[e.ID()])

		case *parse.Set:
			symbols.Globals[e.Name] = assertType(typTable[e.Value.ID()])

		case *parse.ObjectTypeDef:
			symbols.Types[e.Name] = assertType(typTable[e.ID()])

		case *parse.UnionDef:
			symbols.Types[e.Name] = assertType(typTable[e.ID()])

		case *parse.EnumDef:
			symbols.Types[e.Name] = assertType(typTable[e.ID()])

		case *parse.TypeAlias:
			symbols.Types[e.Name] = assertType(typTable[e.ID()])

		case *parse.Import:
		default:
			return moduleExports{}, fmt.Errorf("%w:\n    %s", ErrInvalidTopLevel, expr.Pretty())
		}
	}

	return symbols, nil
}

func assertType(typ TypeScheme) TypeScheme {
	if typ == nil {
		panic("assertion failed: type should not be nil")
	}
	return typ
}

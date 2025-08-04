package simplesub

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/can"
	"github.com/horriblename/typee/src/parse"
)

type ModuleInfo struct {
	Name     can.ModuleName
	Ast      []parse.Expr
	TypesAst []parse.Expr
	Types    map[string]TypeScheme
	Globals  map[string]TypeScheme
	TypeTree map[int]TypeScheme
}

var (
	ErrCyclicImport = errors.New("cyclic import detected")
)

func (self *Typer) typeDeps(program []parse.Expr) error {
	origModule := self.mainModule
	defer func() { self.mainModule = origModule }()

	for _, mod := range program {
		dep, ok := mod.(*parse.Import)
		if !ok {
			break
		}

		name := can.ModuleName(strings.Join(dep.Module, "."))

		if _, ok := self.moduleCache[name]; ok {
			trace("in %s: import %s already type-checked", origModule, self.mainModule)
			continue
		}

		ast, err := parseModule(name)
		if err != nil {
			return fmt.Errorf("parsing module %s: %w", name, err)
		}

		self.mainModule = name
		_, _, err = self.TypeProgram(ast)
		if err != nil {
			return fmt.Errorf("typing module %s: %w", name, err)
		}
	}

	return nil
}

// sort modules in dependency tree, leaf modules (no dependencies) go first

func parseModule(name can.ModuleName) ([]parse.Expr, error) {
	modPath := path.Join(strings.Split(string(name), ".")...) + ".hor"
	data, err := os.ReadFile(modPath)
	if err != nil {
		return nil, fmt.Errorf("error parsing %s: %w", modPath, err)
	}

	return parse.ParseString(string(data))
}

// sorts a map[nodes]outgoingNodes
// leaf nodes go first, root goes last

func typeTableToSymbolMap(mod can.ModuleName, program []parse.Expr, typesAst []parse.Expr, typTable map[int]TypeScheme) (ModuleInfo, error) {
	symbols := ModuleInfo{
		Name:     mod,
		Ast:      program,
		TypesAst: typesAst,
		Types:    map[string]TypeScheme{},
		Globals:  map[string]TypeScheme{},
		TypeTree: typTable,
	}
	for _, expr := range program {
		switch e := expr.(type) {
		case *parse.FuncDef:
			symbols.Globals[e.Name] = assert.Get(typTable, e.ID(),
				fmt.Sprintf("function %s of ID %d is missing from type table", e.Name, e.ID()))

		case *parse.Set:
			symbols.Globals[e.Name] = assert.Get(typTable, e.Value.ID(),
				fmt.Sprintf("function %s of ID %d is missing from type table", e.Name, e.Value.ID()))

		case *parse.ObjectTypeDef:
			symbols.Types[e.Name] = assert.Get(typTable, e.ID(),
				fmt.Sprintf("function %s of ID %d is missing from type table", e.Name, e.ID()))

		case *parse.UnionDef:
			symbols.Types[e.Name] = assert.Get(typTable, e.ID(),
				fmt.Sprintf("function %s of ID %d is missing from type table", e.Name, e.ID()))

		case *parse.EnumDef:
			symbols.Types[e.Name] = assert.Get(typTable, e.ID(),
				fmt.Sprintf("function %s of ID %d is missing from type table", e.Name, e.ID()))

		case *parse.TypeAlias:
			symbols.Types[e.Name] = assert.Get(typTable, e.ID(),
				fmt.Sprintf("function %s of ID %d is missing from type table", e.Name, e.ID()))

		case *parse.Import:
		default:
			return ModuleInfo{}, fmt.Errorf("%w:\n    %s", ErrInvalidTopLevel, expr.Pretty())
		}
	}

	return symbols, nil
}

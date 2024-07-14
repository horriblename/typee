package solve

import (
	"errors"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/opt"
	"github.com/horriblename/typee/src/types"
)

var (
	ErrVariableDefined = errors.New("variable already defined")
)

type TypeID int

type SymbolTable = map[string]types.Type

type ScopeStack struct {
	stack []SymbolTable
}

// stores resolved types, this is the "context" in textbooks
type TypeTable struct {
	ScopeStack    ScopeStack
	Names         map[string]TypeID
	ExprToTypeVar map[ExprID]GeneratedType
	idCounter     TypeID
}

type GeneratedType struct {
	typeVar  TypeVar
	concrete types.Type
}

func (tt *TypeTable) GetTypeVarOfExpr(id ExprID) opt.Option[GeneratedType] {
	if typeVar, ok := tt.ExprToTypeVar[id]; ok {
		return opt.Some(typeVar)
	}

	return opt.None[GeneratedType]()
}

func (tt *TypeTable) SetTypeOfExpr(id ExprID, typeVar GeneratedType) {
	tt.ExprToTypeVar[id] = typeVar
}

func NewTypeTable() TypeTable {
	return TypeTable{
		Names: map[string]TypeID{
			"Int":  0,
			"Str":  1,
			"Bool": 2,
		},
		idCounter: TypeID(2),
	}
}

func (tt TypeTable) Get(name string) TypeID {
	if id, found := tt.Names[name]; found {
		return id
	}

	tt.Names[name] = tt.idCounter
	tt.idCounter++
	return tt.Names[name]
}

func NewScopeStack() *ScopeStack {
	return &ScopeStack{
		stack: []SymbolTable{{}},
	}
}

func (ss *ScopeStack) Find(name string) opt.Option[types.Type] {
	if typ, ok := builtins[name]; ok {
		return opt.Some(typ)
	}

	size := len(ss.stack)
	for i := range ss.stack {
		scope := ss.stack[size-i-1]
		if typ, ok := scope[name]; ok {
			return opt.Some(typ)
		}
	}

	return opt.None[types.Type]()
}

func (ss *ScopeStack) AddScope() { ss.stack = append(ss.stack, SymbolTable{}) }
func (ss *ScopeStack) Pop() {
	assert.GreaterThan(len(ss.stack), 1)
	ss.stack[len(ss.stack)-1] = nil
	ss.stack = ss.stack[:len(ss.stack)-1]
}

func (ss *ScopeStack) DefSymbol(name string, typ types.Type) error {
	assert.GreaterThan(len(ss.stack), 0)

	if _, ok := builtins[name]; ok {
		return ErrVariableDefined
	}

	// disallow variable shadowing
	size := len(ss.stack)
	for i := range ss.stack {
		scope := ss.stack[size-i-1]
		if _, ok := scope[name]; ok {
			return ErrVariableDefined
		}
	}

	ss.stack[len(ss.stack)-1][name] = typ

	return nil
}

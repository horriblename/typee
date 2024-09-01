package solve

import (
	"errors"
	"strings"

	"github.com/horriblename/typee/src/assert"
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
	ScopeStack ScopeStack
}

type GeneratedType struct {
	typeVar  TypeVar
	concrete types.Type
}

func NewTypeTable() TypeTable {
	return TypeTable{ScopeStack: ScopeStack{stack: []SymbolTable{}}}
}

func (ss *ScopeStack) Find(name string) (_ types.Type, found bool) {
	if typ, ok := builtins[name]; ok {
		return typ, true
	}

	size := len(ss.stack)
	for i := range ss.stack {
		scope := ss.stack[size-i-1]
		if typ, ok := scope[name]; ok {
			return typ, true
		}
	}

	return nil, false
}

func (ss *ScopeStack) AddScope() { ss.stack = append(ss.stack, SymbolTable{}) }

func (ss *ScopeStack) Pop() SymbolTable {
	assert.GreaterThan(len(ss.stack), 0)
	last := ss.stack[len(ss.stack)-1]
	ss.stack[len(ss.stack)-1] = nil
	ss.stack = ss.stack[:len(ss.stack)-1]

	return last
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

func (ss *ScopeStack) String() string {
	var b strings.Builder
	b.WriteString("{")

	for _, s := range ss.stack {
		for k, v := range s {
			b.WriteString(k)
			b.WriteString(": ")
			b.WriteString(v.String())
			b.WriteString(", ")
		}
	}

	b.WriteString("}")
	return b.String()
}

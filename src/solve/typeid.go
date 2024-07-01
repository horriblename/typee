package solve

import (
	"github.com/horriblename/typee/src/opt"
	"github.com/horriblename/typee/src/types"
)

type TypeID int

// stores resolved types, this is the "context" in textbooks
type TypeTable struct {
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

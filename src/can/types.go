package can

import (
	"github.com/horriblename/typee/src/opt"
	"github.com/horriblename/typee/src/types"
)

//go-sumtype:decl Type

//
// Types
//

type TypeName string

type Type interface {
	typ()
}

func (AliasType) typ()       {}
func (TypeApplication) typ() {}
func (RecordType) typ()      {}
func (ArrayType) typ()       {}
func (SliceType) typ()       {}
func (FnType) typ()          {}
func (EnumType) typ()        {}
func (ClassType) typ()       {}
func (UnionType) typ()       {}

type AliasType struct {
	Module ModuleID
	Name   string
	Type   Type
	//? Params []struct{Name string, Type}
}

type TypeApplication struct {
	Module ModuleID
	Name   string
	Params []Type
}

type RecordType struct {
	Fields map[string]Type
}

type ArrayType struct {
	Size int64
	Type Type
}

type SliceType struct{ Type Type }

type FnType struct {
	Args []Type
	Ret  Type
}

type EnumType struct {
	Variants []EnumVariant
}

type EnumVariant struct {
	Name  TypeName
	Value opt.Option[int64]
}

type ClassType struct {
	Fields  map[string]Member[Type]
	Methods map[string]Member[FnType]
}

type Member[T any] struct {
	Type   T
	Access types.AccessLvl
}

type UnionType struct {
	Variants []Type
}

//
// Module
//

type Module struct {
	Name    ModuleID
	Aliases map[string]Alias
}

type Alias struct {
	Name string
	Type
	// Params []Type
}

package can

import (
	"github.com/horriblename/typee/src/opt"
	"github.com/horriblename/typee/src/parse"
)

//go-sumtype:decl Type

//
// Types
//

type TypeName string

// defined types
type TypeDef interface {
	typDef()
}

func (TypeAlias) typDef() {}
func (ClassType) typDef() {}
func (EnumType) typDef()  {}
func (UnionType) typDef() {}

// types that are actually "used"
// i.e. not just a definition but a usage
type Type interface {
	typ()
}

func (TypeApplication) typ() {}
func (RecordType) typ()      {}
func (ArrayType) typ()       {}
func (SliceType) typ()       {}
func (FnType) typ()          {}

type TypeAlias struct {
	Module ModuleName
	Name   string
	Type   Type
	//? Params []struct{Name string, Type}
}

type TypeApplication struct {
	Module ModuleName
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
	Access parse.AccessLvl
}

type UnionType struct {
	Variants []Type
}

//
// Module
//

type Module struct {
	Name ModuleName
	Defs map[string]TypeDef
}

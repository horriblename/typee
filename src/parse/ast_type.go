package parse

import (
	"fmt"
	"strings"

	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/opt"
)

//go-sumtype:decl TypeRepr

type TypeRepr interface {
	type_()
	String() string
}

type TypeName struct {
	Module string
	Name   string
}
type SelfType struct{}
type RecordType struct {
	Fields []RecordTypeField
}
type RecordTypeField struct {
	Name string
	Type TypeRepr
}
type ArrayType struct {
	Type TypeRepr
	Size opt.Option[int64]
}
type TypeInstantiation struct {
	Type   TypeName
	Params []TypeRepr
}
type FnType struct {
	Args []TypeRepr
	Ret  TypeRepr
}

func (self TypeName) type_()          {}
func (self SelfType) type_()          {}
func (self RecordType) type_()        {}
func (self ArrayType) type_()         {}
func (self TypeInstantiation) type_() {}
func (self FnType) type_()            {}

func (self TypeName) String() string {
	if self.Module != "" {
		return fmt.Sprintf("%s.%s", self.Module, self.Name)
	}
	return self.Name
}
func (self SelfType) String() string { return "Self" }
func (self RecordType) String() string {
	var b strings.Builder
	b.WriteRune('{')
	for i, field := range self.Fields {
		if i != 0 {
			b.WriteString(", ")
		}
		b.WriteString(field.Name)
		b.WriteString(": ")
		b.WriteString(field.Type.String())
	}
	b.WriteRune('}')
	return b.String()
}
func (self ArrayType) String() string {
	size := ""
	if sz, ok := self.Size.Unwrap(); ok {
		size = fmt.Sprintf(" %d", sz)
	}
	return fmt.Sprintf("[%s%s]", self.Type.String(), size)
}
func (self TypeInstantiation) String() string {
	params := fun.Map(self.Params, func(p TypeRepr) string {
		return p.String()
	})
	return fmt.Sprintf("(%s %s)", self.Type.Name, strings.Join(params, " "))
}
func (self FnType) String() string {
	var b strings.Builder
	b.WriteString("(fn [")
	for i, arg := range self.Args {
		if i != 0 {
			b.WriteString(" ")
		}
		b.WriteString(arg.String())
	}
	b.WriteString("] ")
	b.WriteString(self.Ret.String())
	b.WriteString(")")
	return b.String()
}

func ChildNodes(node TypeRepr) []TypeRepr {
	switch n := node.(type) {
	case ArrayType:
		return []TypeRepr{n.Type}
	case RecordType:
		return fun.Map(n.Fields, func(field RecordTypeField) TypeRepr {
			return field.Type
		})
	case SelfType:
		return []TypeRepr{}
	case TypeInstantiation:
		return append([]TypeRepr{n.Type}, n.Params...)
	case TypeName:
		return []TypeRepr{}
	case FnType:
		return append(n.Args, n.Ret)
	default:
		panic(fmt.Sprintf("unexpected parse.TypeRepr: %#v", node))
	}
}

package parse

import (
	"strings"
)

//go-sumtype:decl TypeRepr

type TypeRepr interface {
	type_()
	String() string
}

type TypeName struct{ Name string }
type SelfType struct{}
type RecordType struct {
	Fields []RecordTypeField
}
type RecordTypeField struct {
	Name string
	Type TypeRepr
}

func (self TypeName) type_()   {}
func (self SelfType) type_()   {}
func (self RecordType) type_() {}

func (self TypeName) String() string { return self.Name }
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

package qbeil

import "fmt"

//go-sumtype:decl Value

type TypedValue struct {
	Type  Type
	Value Value
}

type Value interface {
	val()
	IL() string
}

// global or local variable, $foo/%foo
type Var struct {
	Global bool
	Name   string
}

type IntLiteral struct {
	Value int64
}

func (Var) val()        {}
func (IntLiteral) val() {}

func (v Var) IL() string {
	if v.Global {
		return "$" + v.Name
	} else {
		return "%" + v.Name
	}
}

func (i IntLiteral) IL() string {
	return fmt.Sprintf("%d", i.Value)
}

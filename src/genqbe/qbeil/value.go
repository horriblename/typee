package qbeil

import "fmt"

type Value interface {
	val()
	IL() string
}

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

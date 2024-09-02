package qbeil

import "strconv"

type Value interface {
	val()
	IL() string
}

type Var struct {
	Global bool
	Name   string
}

type IntLiteral struct {
	Value int
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
	return strconv.Itoa(i.Value)
}

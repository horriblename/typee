package qbeil

import "fmt"

//go-sumtype:decl Value

type ABITypedValue struct {
	Type  ABIType
	Value Value
}

type Value interface {
	val()
	IL() string
}

func (Var) val()          {}
func (IntLiteral) val()   {}
func (FloatLiteral) val() {}

// global or local variable, $foo/%foo
type Var struct {
	Global bool
	Name   string
}

type IntLiteral struct {
	Value int64
}

type FloatLiteral struct{ Value float64 } // TODO: support f32

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
func (i FloatLiteral) IL() string {
	return fmt.Sprintf("d_%f", i.Value)
}

func (v Var) String() string { return v.IL() }

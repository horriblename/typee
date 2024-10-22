package biunify

//go-sumtype: TypeNode

type TypeNode interface{ typeNode() }

type Var struct {
	Kind UTypeHead // nilable, only use it's type!!
}
type VNode struct{ Head VTypeHead }
type UNode struct{ Head UTypeHead }

func (Var) typeNode()   {}
func (VNode) typeNode() {}
func (UNode) typeNode() {}

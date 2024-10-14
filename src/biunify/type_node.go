package biunify

//go-sumtype: TypeNode

type TypeNode interface{ typeNode() }

type Var struct{}
type VNode struct{ Head VTypeHead }
type UNode struct{ Head UTypeHead }

func (Var) typeNode()   {}
func (VNode) typeNode() {}
func (UNode) typeNode() {}

type Reachability struct{}

type Edge struct {
	From ID
	To   ID
}

func (self *Reachability) addNode() ID                           { panic("unimpl") }
func (self *Reachability) addEdge(lhs ID, rhs ID, out []Edge) ID { panic("unimpl") }

package parse

import "github.com/horriblename/typee/src/opt"

type Expr interface{ ast() }

// Nodes

type Form struct{ children []Expr }
type Symbol struct{ Name string }
type Int struct{ Value int64 }
type FuncDef struct {
	Name      string
	Signature opt.Option[[]string]
	Args      []string
	Body      []Expr
}
type Set struct {
	Name   string
	rvalue Expr
}
type StrLiteral struct{ Content string }
type IntLiteral struct{ Number int64 }

type FormAttr struct {
}

func (*Form) ast()       {}
func (*Symbol) ast()     {}
func (*Int) ast()        {}
func (*FuncDef) ast()    {}
func (*Set) ast()        {}
func (*StrLiteral) ast() {}
func (*IntLiteral) ast() {}

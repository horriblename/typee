package parse

type Expr interface{ ast() }

type Form struct{ children []Expr }
type Symbol struct{ Name string }
type Int struct{ Value int64 }
type FuncDef struct {
	Name string
	Args []FuncArgDef
	Body []Expr
}
type FuncArgDef struct {
	Name string
	Type string
}
type Set struct {
	Name   string
	rvalue Expr
}
type StrLiteral struct{ Content string }
type IntLiteral struct{ Number int64 }

func (*Form) ast()       {}
func (*Symbol) ast()     {}
func (*Int) ast()        {}
func (*FuncDef) ast()    {}
func (*Set) ast()        {}
func (*StrLiteral) ast() {}
func (*IntLiteral) ast() {}

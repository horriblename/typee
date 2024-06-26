package parse

import (
	"fmt"

	"github.com/horriblename/typee/src/opt"
)

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

type BoolLiteral struct {
	Value bool
}

func (*Form) ast()        {}
func (*Symbol) ast()      {}
func (*Int) ast()         {}
func (*FuncDef) ast()     {}
func (*Set) ast()         {}
func (*StrLiteral) ast()  {}
func (*IntLiteral) ast()  {}
func (*BoolLiteral) ast() {}

func (self *Form) String() string        { return fmt.Sprintf("Form %+v", self.children) }
func (self *Symbol) String() string      { return fmt.Sprintf("Symbol {%s}", self.Name) }
func (self *Int) String() string         { return fmt.Sprintf("Int {%d}", self.Value) }
func (self *FuncDef) String() string     { return fmt.Sprintf("(def %s [%+v])", self.Name, self.Args) }
func (self *Set) String() string         { return fmt.Sprintf("(set %s %+v)", self.Name, self.rvalue) }
func (self *StrLiteral) String() string  { return fmt.Sprintf(`StrLiteral "%s"`, self.Content) }
func (self *IntLiteral) String() string  { return fmt.Sprintf("IntLiteral %d", self.Number) }
func (self *BoolLiteral) String() string { return fmt.Sprintf("BoolLiteral %t", self.Value) }

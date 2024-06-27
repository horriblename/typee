package parse

import (
	"fmt"

	"github.com/horriblename/typee/src/opt"
)

type Expr interface{ ast() }

// Nodes

type Form struct {
	children []Expr
	ID       int
}
type Symbol struct {
	Name string
	ID   int
}
type Int struct {
	Value int64
	ID    int
}
type FuncDef struct {
	ID        int
	Name      string
	Signature opt.Option[[]string]
	Args      []string
	Body      []Expr
}
type Set struct {
	ID     int
	Name   string
	rvalue Expr
}
type StrLiteral struct {
	Content string
	ID      int
}
type IntLiteral struct {
	Number int64
	ID     int
}

type BoolLiteral struct {
	ID    int
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

func (self *Form) String() string   { return fmt.Sprintf("#%d Form %+v", self.ID, self.children) }
func (self *Symbol) String() string { return fmt.Sprintf("#%d Symbol {%s}", self.ID, self.Name) }
func (self *Int) String() string    { return fmt.Sprintf("#%d Int {%d}", self.ID, self.Value) }
func (self *FuncDef) String() string {
	return fmt.Sprintf("#%d (def %s [%+v] %+v)", self.ID, self.Name, self.Args, self.Body)
}
func (self *Set) String() string {
	return fmt.Sprintf("#%d (set %s %+v)", self.ID, self.Name, self.rvalue)
}
func (self *StrLiteral) String() string {
	return fmt.Sprintf(`#%d StrLiteral "%s"`, self.ID, self.Content)
}
func (self *IntLiteral) String() string {
	return fmt.Sprintf("#%d IntLiteral %d", self.ID, self.Number)
}
func (self *BoolLiteral) String() string {
	return fmt.Sprintf("#%d BoolLiteral %t", self.ID, self.Value)
}

package parse

//go-sumtype:decl Expr

import (
	"fmt"

	"github.com/horriblename/typee/src/opt"
)

type Expr interface {
	ast()
	ID() int
}

// Nodes

type Form struct {
	Children []Expr
	id       int
}
type Symbol struct {
	Name string
	id   int
}
type Int struct {
	Value int64
	id    int
}
type FuncDef struct {
	id        int
	Name      string
	Signature opt.Option[[]string]
	Args      []string
	Body      []Expr
}
type Set struct {
	id    int
	Name  string
	Value Expr
}
type IfExpr struct {
	id          int
	Condition   Expr
	Consequence Expr
	Alternative Expr
}
type StrLiteral struct {
	Content string
	id      int
}
type IntLiteral struct {
	Number int64
	id     int
}

type BoolLiteral struct {
	id    int
	Value bool
}

func (*Form) ast()        {}
func (*Symbol) ast()      {}
func (*FuncDef) ast()     {}
func (*Set) ast()         {}
func (*IfExpr) ast()      {}
func (*StrLiteral) ast()  {}
func (*IntLiteral) ast()  {}
func (*BoolLiteral) ast() {}

func (self *Form) ID() int        { return self.id }
func (self *Symbol) ID() int      { return self.id }
func (self *Int) ID() int         { return self.id }
func (self *FuncDef) ID() int     { return self.id }
func (self *Set) ID() int         { return self.id }
func (self *IfExpr) ID() int      { return self.id }
func (self *StrLiteral) ID() int  { return self.id }
func (self *IntLiteral) ID() int  { return self.id }
func (self *BoolLiteral) ID() int { return self.id }

func (self *Form) String() string   { return fmt.Sprintf("#%d Form %+v", self.id, self.Children) }
func (self *Symbol) String() string { return fmt.Sprintf("#%d Symbol {%s}", self.id, self.Name) }
func (self *Int) String() string    { return fmt.Sprintf("#%d Int {%d}", self.id, self.Value) }
func (self *FuncDef) String() string {
	return fmt.Sprintf("#%d (def %s [%+v] %+v)", self.id, self.Name, self.Args, self.Body)
}
func (self *Set) String() string {
	return fmt.Sprintf("#%d (set %s %+v)", self.id, self.Name, self.Value)
}
func (self *IfExpr) String() string {
	return fmt.Sprintf("#%d (if %v %v %v)", self.id, self.Condition, self.Consequence, self.Alternative)
}
func (self *StrLiteral) String() string {
	return fmt.Sprintf(`#%d StrLiteral "%s"`, self.id, self.Content)
}
func (self *IntLiteral) String() string {
	return fmt.Sprintf("#%d IntLiteral %d", self.id, self.Number)
}
func (self *BoolLiteral) String() string {
	return fmt.Sprintf("#%d BoolLiteral %t", self.id, self.Value)
}

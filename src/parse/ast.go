package parse

//go-sumtype:decl Expr

import (
	"fmt"
	"strings"

	"github.com/horriblename/typee/src/opt"
)

type Expr interface {
	ast()
	ID() int
	Pretty() string
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

type Assignment struct {
	Var   string
	Value Expr
}

type LetExpr struct {
	id          int
	Assignments []Assignment
	Body        Expr
}

type Fn struct {
	id   int
	Arg  string
	Body Expr
}

type Record struct {
	id     int
	Fields []RecordField
}

type RecordField struct {
	Name  string
	Value Expr
}

func (*Form) ast()        {}
func (*Symbol) ast()      {}
func (*FuncDef) ast()     {}
func (*Set) ast()         {}
func (*IfExpr) ast()      {}
func (*StrLiteral) ast()  {}
func (*IntLiteral) ast()  {}
func (*BoolLiteral) ast() {}
func (*LetExpr) ast()     {}
func (*Fn) ast()          {}
func (*Record) ast()      {}

func (self *Form) ID() int        { return self.id }
func (self *Symbol) ID() int      { return self.id }
func (self *Int) ID() int         { return self.id }
func (self *FuncDef) ID() int     { return self.id }
func (self *Set) ID() int         { return self.id }
func (self *IfExpr) ID() int      { return self.id }
func (self *StrLiteral) ID() int  { return self.id }
func (self *IntLiteral) ID() int  { return self.id }
func (self *BoolLiteral) ID() int { return self.id }
func (self *LetExpr) ID() int     { return self.id }
func (self *Fn) ID() int          { return self.id }
func (self *Record) ID() int      { return self.id }

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
	return fmt.Sprintf("#%d (if [%v] %v %v)", self.id, self.Condition, self.Consequence, self.Alternative)
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
func (self *LetExpr) String() string {
	return fmt.Sprintf("#%d (let %v %v)", self.id, self.Assignments, self.Body)
}
func (self *Fn) String() string {
	return fmt.Sprintf("#%d (fn [%v] %v)", self.id, self.Arg, self.Body)
}
func (self *Record) String() string {
	return fmt.Sprintf("#%d %v", self.id, self.Fields)
}
func (self *RecordField) String() string {
	return fmt.Sprintf("%s: %s", self.Name, self.Value)
}

func prettySlice(xs []Expr) []string {
	ys := make([]string, 0, len(xs))
	for _, x := range xs {
		ys = append(ys, x.Pretty())
	}

	return ys
}

func (self *Form) Pretty() string   { return fmt.Sprintf("(%v)", prettySlice(self.Children)) }
func (self *Symbol) Pretty() string { return fmt.Sprintf("%s", self.Name) }
func (self *Int) Pretty() string    { return fmt.Sprintf("%d", self.Value) }
func (self *FuncDef) Pretty() string {
	return fmt.Sprintf("(def %s [%v] %v)", self.Name, self.Args, prettySlice(self.Body))
}
func (self *Set) Pretty() string {
	return fmt.Sprintf("(set %s %v)", self.Name, self.Value.Pretty())
}
func (self *IfExpr) Pretty() string {
	return fmt.Sprintf("(if [%v] %v %v)", self.Condition.Pretty(), self.Consequence.Pretty(), self.Alternative.Pretty())
}
func (self *StrLiteral) Pretty() string {
	return fmt.Sprintf(`"%s"`, self.Content)
}
func (self *IntLiteral) Pretty() string {
	return fmt.Sprintf("%d", self.Number)
}
func (self *BoolLiteral) Pretty() string {
	return fmt.Sprintf("%t", self.Value)
}
func (self *LetExpr) Pretty() string {
	var b strings.Builder
	b.WriteString("(let [")
	for _, ass := range self.Assignments {
		b.WriteString(ass.Var)
		b.WriteString(" ")
		b.WriteString(ass.Value.Pretty())
		b.WriteString(" ")
	}
	b.WriteString("]")
	b.WriteString(self.Body.Pretty())
	b.WriteString(")")
	return b.String()
}
func (self *Fn) Pretty() string {
	return fmt.Sprintf("(fn [%s] %s)", self.Arg, self.Body.Pretty())
}
func (self *Record) Pretty() string {
	if len(self.Fields) == 0 {
		return "{}"
	}

	var b strings.Builder
	b.WriteString("{")
	b.WriteString(self.Fields[0].Name)
	b.WriteRune(':')
	b.WriteString(self.Fields[0].Value.Pretty())

	for _, field := range self.Fields[1:] {
		b.WriteString(", ")
		b.WriteString(field.Name)
		b.WriteRune(':')
		b.WriteString(field.Value.Pretty())
	}
	b.WriteString("}")
	return b.String()
}

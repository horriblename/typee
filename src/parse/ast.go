package parse

//go-sumtype:decl Expr

import (
	"fmt"
	"strings"

	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/opt"
	"github.com/horriblename/typee/src/types"
)

type Expr interface {
	ast()
	ID() int
	String() string
	Pretty() string
	ChildNodes() []Expr
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
type FuncDef struct {
	id        int
	Name      string
	Signature opt.Option[[]TypeRepr]
	Args      []string
	Body      []Expr
}
type Set struct {
	id    int
	Name  string
	Value Expr
}
type VarDef struct {
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

type SelfLiteral struct{ id int }

type Assignment struct {
	Var   string
	Value Expr
}

type LetExpr struct {
	id          int
	Recursive   bool // TODO: split into separate node type
	Assignments []Assignment
	Body        Expr
}

type TaggedExpr struct {
	id   int
	Tag  string
	Body Expr
}

type Fn struct {
	Id        int // export id since Fn is used as a desugar target
	Signature opt.Option[[]TypeRepr]
	Args      []string
	Body      Expr
}

type CaseExpr struct {
	id       int
	Match    Expr
	Branches []CaseBranch
}

type CaseBranch struct {
	Pattern CasePattern
	Body    Expr
}

type CasePattern struct {
	Tag     string
	Pattern string
}

type Record struct {
	id     int
	Fields []RecordField
}

type RecordAccess struct {
	id     int
	Record Expr
	Field  string
}

type RecordField struct {
	Name  string
	Value Expr
}

type ArrayLiteral struct {
	id       int
	Elements []Expr
}

type ClassDef struct {
	id     int
	Name   string
	Supers []string
	Fields []ClassMember
}

type InterfaceDef struct {
	id     int
	Name   string
	Supers []string
	Fields []ClassMember
}

type MethodAccess struct {
	id     int
	Var    Expr
	Method string
}

type New struct {
	id    int
	Class string
}

type UnionDef struct {
	id       int
	Name     string
	Variants []TypeRepr
}

type EnumDef struct {
	id       int
	Name     string
	Variants []EnumVariant
}

type EnumVariant struct {
	Name  string
	Value opt.Option[int64]
	// Payload TypeRepr
}

type EnumAccess struct {
	id   int
	Enum string
	Key  string
}

type TypeAlias struct {
	id   int
	Name string
	Type TypeRepr
}

type ExternCall struct {
	id     int
	Symbol Symbol
	Args   []Expr
}

func (*Form) ast()         {}
func (*Symbol) ast()       {}
func (*FuncDef) ast()      {}
func (*Set) ast()          {}
func (*VarDef) ast()       {}
func (*IfExpr) ast()       {}
func (*StrLiteral) ast()   {}
func (*IntLiteral) ast()   {}
func (*BoolLiteral) ast()  {}
func (*SelfLiteral) ast()  {}
func (*LetExpr) ast()      {}
func (*TaggedExpr) ast()   {}
func (*Fn) ast()           {}
func (*CaseExpr) ast()     {}
func (*Record) ast()       {}
func (*ArrayLiteral) ast() {}
func (*ClassDef) ast()     {}
func (*InterfaceDef) ast() {}
func (*RecordAccess) ast() {}
func (*MethodAccess) ast() {}
func (*New) ast()          {}
func (*UnionDef) ast()     {}
func (*EnumDef) ast()      {}
func (*EnumAccess) ast()   {}
func (*TypeAlias) ast()    {}
func (*ExternCall) ast()   {}

func (self *Form) ID() int         { return self.id }
func (self *Symbol) ID() int       { return self.id }
func (self *FuncDef) ID() int      { return self.id }
func (self *Set) ID() int          { return self.id }
func (self *VarDef) ID() int       { return self.id }
func (self *IfExpr) ID() int       { return self.id }
func (self *StrLiteral) ID() int   { return self.id }
func (self *IntLiteral) ID() int   { return self.id }
func (self *BoolLiteral) ID() int  { return self.id }
func (self *SelfLiteral) ID() int  { return self.id }
func (self *LetExpr) ID() int      { return self.id }
func (self *TaggedExpr) ID() int   { return self.id }
func (self *Fn) ID() int           { return self.Id }
func (self *CaseExpr) ID() int     { return self.id }
func (self *Record) ID() int       { return self.id }
func (self *ArrayLiteral) ID() int { return self.id }
func (self *ClassDef) ID() int     { return self.id }
func (self *InterfaceDef) ID() int { return self.id }
func (self *RecordAccess) ID() int { return self.id }
func (self *MethodAccess) ID() int { return self.id }
func (self *New) ID() int          { return self.id }
func (self *UnionDef) ID() int     { return self.id }
func (self *EnumDef) ID() int      { return self.id }
func (self *EnumAccess) ID() int   { return self.id }
func (self *TypeAlias) ID() int    { return self.id }
func (self *ExternCall) ID() int   { return self.id }

func (self *Form) String() string   { return fmt.Sprintf("#%d Form %+v", self.id, self.Children) }
func (self *Symbol) String() string { return fmt.Sprintf("#%d Symbol {%s}", self.id, self.Name) }
func (self *FuncDef) String() string {
	sigStr := ""
	if sig, ok := self.Signature.Unwrap(); ok {
		sigStr = " (" + strings.Join(fun.Map(sig, func(tr TypeRepr) string { return tr.String() }), " ") + ")"
	}
	return fmt.Sprintf("#%d (def %s%s [%+v] %+v)", self.id, self.Name, sigStr, self.Args, self.Body)
}
func (self *Set) String() string {
	return fmt.Sprintf("#%d (set %s %+v)", self.id, self.Name, self.Value)
}
func (self *VarDef) String() string {
	return fmt.Sprintf("#%d (var %s %+v)", self.id, self.Name, self.Value)
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
func (self *SelfLiteral) String() string {
	return fmt.Sprintf("#%d SelfLiteral", self.id)
}
func (self *LetExpr) String() string {
	keyword := "let"
	if self.Recursive {
		keyword = "letrec"
	}
	return fmt.Sprintf("#%d (%s %v %v)", self.id, keyword, self.Assignments, self.Body)
}
func (self *TaggedExpr) String() string {
	return fmt.Sprintf("#%d ('%s %v)", self.id, self.Tag, self.Body)
}
func (self *Fn) String() string {
	return fmt.Sprintf("#%d (fn [%v] %v)", self.Id, self.Args, self.Body)
}
func (self *CaseExpr) String() string {
	return fmt.Sprintf("#%d (case %v [%s])", self.id, self.Match, strings.Join(fun.Map(self.Branches, func(branch CaseBranch) string {
		return branch.String()
	}), " "))
}
func (self *CaseBranch) String() string {
	return fmt.Sprintf("('%s %s) %v", self.Pattern.Tag, self.Pattern.Pattern, self.Body)
}
func (self *Record) String() string {
	return fmt.Sprintf("#%d %v", self.id, self.Fields)
}
func (self *ArrayLiteral) String() string {
	return fmt.Sprintf("#%d %+v", self.id, self.Elements)
}
func (self *ClassDef) String() string {
	return fmt.Sprintf("#%d (class %s %v %v)", self.id, self.Name, self.Supers, self.Fields)
}
func (self *InterfaceDef) String() string {
	return fmt.Sprintf("#%d (interface %s %v {%v})", self.id, self.Name, self.Supers, self.Fields)
}
func (self *RecordAccess) String() string {
	return fmt.Sprintf("#%d %s.%s", self.id, self.Record.String(), self.Field)
}
func (self *MethodAccess) String() string {
	return fmt.Sprintf("#%d %s.%s", self.id, self.Var.String(), self.Method)
}
func (self *RecordField) String() string {
	return fmt.Sprintf("%s: %s", self.Name, self.Value)
}
func (self *New) String() string {
	return fmt.Sprintf("%s.new", self.Class)
}
func (self *UnionDef) String() string {
	variants := fun.Map(self.Variants, func(t TypeRepr) string { return t.String() })
	return fmt.Sprintf("#%d (enum %s {\n%s\n})", self.id, self.Name, strings.Join(variants, "\n"))
}
func (self *EnumDef) String() string {
	variants := fun.Map(self.Variants, func(t EnumVariant) string { return t.String() })
	return fmt.Sprintf("#%d (enum %s {\n%s\n})", self.id, self.Name, strings.Join(variants, "\n"))
}
func (self *EnumAccess) String() string {
	return fmt.Sprintf("#%d%s::%s", self.id, self.Enum, self.Key)
}
func (self *TypeAlias) String() string {
	return fmt.Sprintf("#%d(type %s %s)", self.id, self.Name, self.Type.String())
}
func (self *ExternCall) String() string {
	args := fun.Map(self.Args, func(e Expr) string { return e.String() })
	return fmt.Sprintf("#%d(callExtern %s %s)", self.id, self.Symbol.Name, strings.Join(args, " "))
}
func (self *EnumVariant) String() string {
	if val, ok := self.Value.Unwrap(); ok {
		return fmt.Sprintf("%s: %d", self.Name, val)
	}
	return self.Name
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
func (self *FuncDef) Pretty() string {
	sigStr := ""
	if sig, ok := self.Signature.Unwrap(); ok {
		sigStr = " (" + strings.Join(fun.Map(sig, func(tr TypeRepr) string { return tr.String() }), " ") + ")"
	}
	return fmt.Sprintf("(def %s%s [%v] %v)", self.Name, sigStr, self.Args, prettySlice(self.Body))
}
func (self *Set) Pretty() string {
	return fmt.Sprintf("(set %s %v)", self.Name, self.Value.Pretty())
}
func (self *VarDef) Pretty() string {
	return fmt.Sprintf("(var %s %v)", self.Name, self.Value.Pretty())
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
func (self *SelfLiteral) Pretty() string {
	return fmt.Sprintf("self")
}
func (self *LetExpr) Pretty() string {
	var b strings.Builder
	if self.Recursive {
		b.WriteString("(letrec [")
	} else {
		b.WriteString("(let [")
	}
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
func (self *TaggedExpr) Pretty() string {
	return fmt.Sprintf("('%s %s)", self.Tag, self.Body.Pretty())
}
func (self *Fn) Pretty() string {
	return fmt.Sprintf("(fn [%s] %s)", self.Args, self.Body.Pretty())
}
func (self *CaseExpr) Pretty() string {
	var b strings.Builder
	b.WriteString("(case ")
	b.WriteString(self.Match.Pretty())
	b.WriteString(" [")
	for _, branch := range self.Branches {
		b.WriteString(branch.Pretty())
	}
	b.WriteString(" ]")
	return b.String()
}
func (self *CaseBranch) Pretty() string {
	return fmt.Sprintf("('%s %s) %s", self.Pattern.Tag, self.Pattern.Pattern, self.Body.Pretty())
}
func (self *RecordAccess) Pretty() string {
	return fmt.Sprintf("%s.%s", self.Record.Pretty(), self.Field)
}
func (self *MethodAccess) Pretty() string {
	return fmt.Sprintf("%s#%s", self.Var.Pretty(), self.Method)
}
func (self *New) Pretty() string {
	return fmt.Sprintf("%s.new", self.Class)
}
func (self *UnionDef) Pretty() string {
	variants := fun.Map(self.Variants, func(t TypeRepr) string { return t.String() })
	return fmt.Sprintf("(union %s {\n%s\n})", self.Name, strings.Join(variants, "\n"))
}
func (self *EnumDef) Pretty() string {
	variants := fun.Map(self.Variants, func(t EnumVariant) string { return t.String() })
	return fmt.Sprintf("(enum %s {\n%s\n})", self.Name, strings.Join(variants, "\n"))
}
func (self *EnumAccess) Pretty() string {
	return fmt.Sprintf("%s::%s", self.Enum, self.Key)
}
func (self *TypeAlias) Pretty() string {
	return fmt.Sprintf("(type %s %s)", self.Name, self.Type)
}
func (self *ExternCall) Pretty() string {
	args := fun.Map(self.Args, func(e Expr) string { return e.Pretty() })
	return fmt.Sprintf("(callExtern %s %s)", self.Symbol.Name, strings.Join(args, " "))
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
func (self *ArrayLiteral) Pretty() string {
	el := fun.Map(self.Elements, func(e Expr) string { return e.Pretty() })
	return fmt.Sprintf("%+s", el)
}

func (self *ClassDef) Pretty() string {
	if len(self.Fields) == 0 {
		return "{}"
	}

	var b strings.Builder
	b.WriteString("class ")
	b.WriteString(self.Name)
	b.WriteString(" ")
	b.WriteString("(")
	b.WriteString(strings.Join(self.Supers, ","))
	b.WriteString(") {")
	b.WriteString(strings.Join(
		fun.Map(self.Fields, func(m ClassMember) string { return m.String() }),
		", "))
	b.WriteString("}")
	return b.String()
}

func (self *InterfaceDef) Pretty() string {
	if len(self.Fields) == 0 {
		return "{}"
	}

	var b strings.Builder
	b.WriteString("class ")
	b.WriteString(self.Name)
	b.WriteString("(")
	b.WriteString(strings.Join(self.Supers, ","))
	b.WriteString(") {")
	b.WriteString(strings.Join(
		fun.Map(self.Fields, func(m ClassMember) string { return m.String() }),
		", "))
	b.WriteString("}")
	return b.String()
}

// class member

type ClassMember interface {
	Access() types.AccessLvl
	Name() string
	String() string
}

type ClassField struct {
	Access_ types.AccessLvl
	Name_   string
	Type    TypeRepr
}

type ClassMethod struct {
	Access_ types.AccessLvl
	Func    *FuncDef
}

func (self ClassField) Access() types.AccessLvl  { return self.Access_ }
func (self ClassMethod) Access() types.AccessLvl { return self.Access_ }

func (self ClassField) Name() string  { return self.Name_ }
func (self ClassMethod) Name() string { return self.Func.Name }

func (self ClassField) String() string {
	return fmt.Sprintf("%s %s %s", self.Access_, self.Name_, self.Type.String())
}
func (self ClassMethod) String() string {
	return fmt.Sprintf("%s %s", self.Access(), self.Func.String())
}

// TODO: rename to Subexpressions
func (self *Form) ChildNodes() []Expr {
	return self.Children
}
func (self *Symbol) ChildNodes() []Expr  { return []Expr{} }
func (self *FuncDef) ChildNodes() []Expr { return self.Body }
func (self *Set) ChildNodes() []Expr     { return []Expr{self.Value} }
func (self *VarDef) ChildNodes() []Expr  { return []Expr{self.Value} }
func (self *IfExpr) ChildNodes() []Expr {
	return []Expr{self.Condition, self.Consequence, self.Alternative}
}
func (self *StrLiteral) ChildNodes() []Expr  { return []Expr{} }
func (self *IntLiteral) ChildNodes() []Expr  { return []Expr{} }
func (self *BoolLiteral) ChildNodes() []Expr { return []Expr{} }
func (self *SelfLiteral) ChildNodes() []Expr { return []Expr{} }
func (self *LetExpr) ChildNodes() []Expr {
	ass := fun.Map(self.Assignments, func(ass Assignment) Expr {
		return ass.Value
	})
	return append(ass, self.Body)
}
func (self *TaggedExpr) ChildNodes() []Expr { return []Expr{self.Body} }
func (self *Fn) ChildNodes() []Expr         { return []Expr{self.Body} }
func (self *CaseExpr) ChildNodes() []Expr {
	c := make([]Expr, 0, len(self.Branches)+1)
	c = append(c, self.Match)
	for _, branch := range self.Branches {
		c = append(c, branch.Body)
	}
	return c
}
func (self *Record) ChildNodes() []Expr {
	return fun.Map(self.Fields, func(f RecordField) Expr {
		return f.Value
	})
}
func (self *ArrayLiteral) ChildNodes() []Expr {
	return self.Elements
}
func (self *ClassDef) ChildNodes() []Expr {
	c := []Expr{}
	for _, member := range self.Fields {
		if meth, ok := member.(ClassMethod); ok {
			c = append(c, meth.Func)
		}
	}
	return c
}
func (self *InterfaceDef) ChildNodes() []Expr {
	c := []Expr{}
	for _, member := range self.Fields {
		if meth, ok := member.(ClassMethod); ok {
			c = append(c, meth.Func)
		}
	}
	return c
}
func (self *RecordAccess) ChildNodes() []Expr { return []Expr{self.Record} }
func (self *MethodAccess) ChildNodes() []Expr { return []Expr{self.Var} }
func (self *New) ChildNodes() []Expr          { return []Expr{} }
func (self *UnionDef) ChildNodes() []Expr     { return []Expr{} }
func (self *EnumDef) ChildNodes() []Expr      { return []Expr{} }
func (self *EnumAccess) ChildNodes() []Expr   { return []Expr{} }
func (self *TypeAlias) ChildNodes() []Expr    { return []Expr{} }
func (self *ExternCall) ChildNodes() []Expr   { return append([]Expr{&self.Symbol}, self.Args...) }

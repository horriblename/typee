package lex

//go-sumtype:decl Token

import "fmt"

type Token interface {
	token()
	String() string
}

type LParen struct{}
type RParen struct{}
type LBracket struct{}
type RBracket struct{}
type LBrace struct{}
type RBrace struct{}
type Colon struct{}
type Comma struct{}
type Dot struct{}
type Hash struct{}
type Symbol struct {
	Name string
}
type IntLiteral struct {
	Number int64
}
type StrLiteral struct {
	Content string
}
type Tag struct {
	Label string
}
type TrueLiteral struct{}
type FalseLiteral struct{}

// keywords
type Def struct{}
type Set struct{}
type Var struct{}
type If struct{}
type Let struct{}
type LetRec struct{}
type Fn struct{}
type Case struct{}
type Class struct{}
type Interface struct{}
type Pub struct{}
type Protected struct{}
type Priv struct{}
type Self struct{}
type SelfType struct{}
type New struct{}
type Union struct{}
type Enum struct{}

func (*LParen) token()       {}
func (*RParen) token()       {}
func (*LBracket) token()     {}
func (*RBracket) token()     {}
func (*LBrace) token()       {}
func (*RBrace) token()       {}
func (*Colon) token()        {}
func (*Comma) token()        {}
func (*Dot) token()          {}
func (*Hash) token()         {}
func (*Symbol) token()       {}
func (*IntLiteral) token()   {}
func (*StrLiteral) token()   {}
func (*Tag) token()          {}
func (*Def) token()          {}
func (*Set) token()          {}
func (*Var) token()          {}
func (*If) token()           {}
func (*Let) token()          {}
func (*LetRec) token()       {}
func (*Fn) token()           {}
func (*Case) token()         {}
func (*Class) token()        {}
func (*Interface) token()    {}
func (*Pub) token()          {}
func (*Protected) token()    {}
func (*Priv) token()         {}
func (*Self) token()         {}
func (*SelfType) token()     {}
func (*New) token()          {}
func (*Union) token()        {}
func (*Enum) token()         {}
func (*TrueLiteral) token()  {}
func (*FalseLiteral) token() {}

func (*LParen) String() string          { return "LParen" }
func (*RParen) String() string          { return "RParen" }
func (*LBracket) String() string        { return "LBracket" }
func (*RBracket) String() string        { return "RBracket" }
func (*LBrace) String() string          { return "LBrace" }
func (*RBrace) String() string          { return "RBrace" }
func (*Colon) String() string           { return "Colon" }
func (*Comma) String() string           { return "Comma" }
func (*Dot) String() string             { return "Dot" }
func (*Hash) String() string            { return "Hash" }
func (self *Symbol) String() string     { return fmt.Sprintf("Symbol{\"%s\"}", self.Name) }
func (self *IntLiteral) String() string { return fmt.Sprintf("IntLiteral{%d}", self.Number) }
func (self *StrLiteral) String() string { return fmt.Sprintf("StrLiteral{\"%s\"}", self.Content) }
func (self *Tag) String() string        { return fmt.Sprintf("'%s", self.Label) }
func (*Def) String() string             { return "Def" }
func (*Set) String() string             { return "Set" }
func (*Var) String() string             { return "Var" }
func (*If) String() string              { return "If" }
func (*Let) String() string             { return "let" }
func (*LetRec) String() string          { return "letrec" }
func (*Fn) String() string              { return "fn" }
func (*Case) String() string            { return "case" }
func (*Class) String() string           { return "class" }
func (*Interface) String() string       { return "interface" }
func (*Pub) String() string             { return "pub" }
func (*Protected) String() string       { return "protected" }
func (*Priv) String() string            { return "priv" }
func (*Self) String() string            { return "self" }
func (*SelfType) String() string        { return "Self" }
func (*New) String() string             { return "new" }
func (*Union) String() string           { return "union" }
func (*Enum) String() string            { return "enum" }
func (*TrueLiteral) String() string     { return "true" }
func (*FalseLiteral) String() string    { return "false" }

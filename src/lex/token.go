package lex

import "fmt"

type Token interface {
	token()
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
type If struct{}
type Let struct{}
type Fn struct{}
type Case struct{}

func (*LParen) token()       {}
func (*RParen) token()       {}
func (*LBracket) token()     {}
func (*RBracket) token()     {}
func (*LBrace) token()       {}
func (*RBrace) token()       {}
func (*Colon) token()        {}
func (*Comma) token()        {}
func (*Dot) token()          {}
func (*Symbol) token()       {}
func (*IntLiteral) token()   {}
func (*StrLiteral) token()   {}
func (*Tag) token()          {}
func (*Def) token()          {}
func (*Set) token()          {}
func (*If) token()           {}
func (*Let) token()          {}
func (*Fn) token()           {}
func (*Case) token()         {}
func (*TrueLiteral) token()  {}
func (*FalseLiteral) token() {}

func (*LParen) String() string          { return "LParen" }
func (*RParen) String() string          { return "RParen" }
func (*LBracket) String() string        { return "LBracket" }
func (*RBracket) String() string        { return "RBracket" }
func (*LBrace) String() string          { return "LBrace" }
func (*RBrace) String() string          { return "RBrace" }
func (*Colon) String() string           { return "Colon" }
func (*Dot) String() string             { return "Dot" }
func (self *Symbol) String() string     { return fmt.Sprintf("Symbol{\"%s\"}", self.Name) }
func (self *IntLiteral) String() string { return fmt.Sprintf("IntLiteral{%d}", self.Number) }
func (self *StrLiteral) String() string { return fmt.Sprintf("StrLiteral{\"%s\"}", self.Content) }
func (self *Tag) String() string        { return fmt.Sprintf("'%s", self.Label) }
func (*Def) String() string             { return "Def" }
func (*Set) String() string             { return "Set" }
func (*If) String() string              { return "If" }
func (*Let) String() string             { return "let" }
func (*Fn) String() string              { return "fn" }
func (*Case) String() string            { return "case" }
func (*TrueLiteral) String() string     { return "true" }
func (*FalseLiteral) String() string    { return "false" }

package lex

import "fmt"

type Token interface {
	token()
}

type LParen struct{}
type RParen struct{}
type Colon struct{}
type Symbol struct {
	Name string
}
type IntLiteral struct {
	Number int64
}
type StrLiteral struct {
	Content string
}

// keywords
type Def struct{}
type Set struct{}

func (*LParen) token()     {}
func (*RParen) token()     {}
func (*Colon) token()      {}
func (*Symbol) token()     {}
func (*IntLiteral) token() {}
func (*StrLiteral) token() {}
func (*Def) token()        {}
func (*Set) token()        {}

func (*LParen) String() string          { return "LParen" }
func (*RParen) String() string          { return "RParen" }
func (*Colon) String() string           { return "Colon" }
func (self *Symbol) String() string     { return fmt.Sprintf("Symbol{\"%s\"}", self.Name) }
func (*IntLiteral) String() string      { return "IntLiteral" }
func (self *StrLiteral) String() string { return fmt.Sprintf("StrLiteral{\"%s\"}", self.Content) }
func (*Def) String() string             { return "Def" }
func (*Set) String() string             { return "Set" }

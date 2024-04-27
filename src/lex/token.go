package lex

type Token interface {
	token()
}

type LParen struct{}
type RParen struct{}
type Symbol struct {
	String string
}
type IntLiteral struct {
	Number int64
}

func (*LParen) token()     {}
func (*RParen) token()     {}
func (*Symbol) token()     {}
func (*IntLiteral) token() {}

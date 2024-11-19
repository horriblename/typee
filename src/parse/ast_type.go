package parse

//go-sumtype:decl TypeRepr

type TypeRepr interface {
	type_()
	String() string
}

type TypeName struct{ Name string }
type SelfType struct{}

func (self TypeName) type_() {}
func (self SelfType) type_() {}

func (self TypeName) String() string { return self.Name }
func (self SelfType) String() string { return "Self" }

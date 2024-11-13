package parse

//go-sumtype:decl TypeRepr

type TypeRepr interface {
	type_()
	String() string
}

type TypeName struct{ Name string }

func (self TypeName) type_() {}

func (self TypeName) String() string { return self.Name }

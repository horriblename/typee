package types

type Type interface {
	type_()
}

type String struct{}
type Int struct{}
type Bool struct{}
type Func struct {
	args []Type
	ret  Type
}

func (*String) type_() {}
func (*Int) type_()    {}
func (*Bool) type_()   {}
func (*Func) type_()   {}

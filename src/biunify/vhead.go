package biunify

//go-sumtype:decl VTypeHead UTypeHead

type ID = int

type VTypeHead interface {
	vTypeHead()
}

type VBool struct{}
type VFunc struct {
	Arg []Use
	Ret Value
}
type VObj struct{ Fields map[string]Value }
type VTagged NamedValue

func (VBool) vTypeHead()   {}
func (VFunc) vTypeHead()   {}
func (VObj) vTypeHead()    {}
func (VTagged) vTypeHead() {}

type UTypeHead interface {
	uTypeHead()
}

type UBool struct{}
type UFunc struct {
	Arg []Value
	Ret Use
}
type UObj struct{ Field NamedUse }
type UTagged struct{ Variants map[string]Use }

func (UBool) uTypeHead()   {}
func (UFunc) uTypeHead()   {}
func (UObj) uTypeHead()    {}
func (UTagged) uTypeHead() {}

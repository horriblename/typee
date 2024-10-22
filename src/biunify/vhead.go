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

func MatchTypeHead(v VTypeHead, u UTypeHead) bool {
	return is[VBool](v) && is[UBool](u) ||
		is[VFunc](v) && is[UFunc](u) ||
		is[VObj](v) && is[UObj](u) ||
		is[VTagged](v) && is[UTagged](u)
}

func is[T any](x interface{}) bool {
	_, ok := x.(T)
	return ok
}

package types

import (
	"fmt"
	"strings"

	"github.com/horriblename/typee/src/assert"
)

type Type interface {
	type_()
	String() string
	Simple() bool
	Eq(other Type) bool
}

type TypeID int

type String struct{}
type Int struct{}
type Bool struct{}
type Func struct {
	Args []Type
	Ret  Type
}
type Generic struct {
	ID      TypeID
	Name    string // can be empty
	Comment string // can be empty
}

func (*String) type_()  {}
func (*Int) type_()     {}
func (*Bool) type_()    {}
func (*Func) type_()    {}
func (*Generic) type_() {}

func (*String) Simple() bool  { return true }
func (*Int) Simple() bool     { return true }
func (*Bool) Simple() bool    { return true }
func (*Func) Simple() bool    { return false }
func (*Generic) Simple() bool { return false }

func (*String) Eq(other Type) bool {
	_, ok := other.(*String)
	return ok
}
func (*Int) Eq(other Type) bool {
	_, ok := other.(*Int)
	return ok
}
func (*Bool) Eq(other Type) bool {
	_, ok := other.(*Bool)
	return ok
}
func (*Func) Eq(other Type) bool { panic("TODO") }
func (g *Generic) Eq(other Type) bool {
	o, ok := other.(*Generic)
	return ok && o.ID == g.ID
}

func (*String) String() string { return "String" }
func (*Int) String() string    { return "Int" }
func (*Bool) String() string   { return "Bool" }
func (f *Func) String() string {
	assert.True(len(f.Args) > 0)
	b := strings.Builder{}
	b.WriteString(f.Args[0].String())
	for _, arg := range f.Args[1:] {
		b.WriteString(" , ")
		b.WriteString(arg.String())
	}

	b.WriteString(" -> ")
	b.WriteString(f.Ret.String())

	return b.String()
}
func (g *Generic) String() string { return fmt.Sprintf("t#%d", g.ID) }

var genericIDCounter TypeID = 0

func NewGeneric(name string, comment string) *Generic {
	genericIDCounter++
	return &Generic{
		ID:      genericIDCounter,
		Name:    name,
		Comment: comment,
	}
}

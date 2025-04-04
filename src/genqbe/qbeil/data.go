package qbeil

import (
	"fmt"
	"strconv"
	"strings"
)

type DataDef struct {
	Linkage Linkage
	VarName string
	Align   int // 0 for auto
}

// rename pls
type DataField interface {
	data()
	IL() string
}

func (Zeros) data()     {}
func (DataItems) data() {}

type Zeros struct{ Count int }

func (self Zeros) IL() string {
	return fmt.Sprintf("z %d", self.Count)
}

// NOTE: only the {w 0, w 1} form is supported
// the {w 0 1} shorthand is not
type DataItems []TypedDataItem

func (self DataItems) IL() string {
	var b strings.Builder
	for _, item := range self {
		b.WriteString(item.Type.IL())
		b.WriteString(" ")
		b.WriteString(item.Value.IL())
		b.WriteString(", ")
	}
	return b.String()
}

type TypedDataItem struct {
	Type  ExtType
	Value DataItem
}

type DataItem interface {
	dataItem()
	IL() string
}

// only global variables allowed, so, uh, don't mess up?
func (Var) dataItem()          {}
func (StrDataItem) dataItem()  {}
func (FloatLiteral) dataItem() {}
func (IntLiteral) dataItem()   {}

type StrDataItem string

func (self StrDataItem) IL() string {
	return strconv.Quote(string(self))
}

func (b *Builder) CompositeData(def DataDef, field DataField) Var {
	b.dataPrelude(def)

	b.Buf.WriteString("{")
	b.Buf.WriteString(field.IL())
	b.Buf.WriteString("}\n")

	return Var{Global: true, Name: def.VarName}
}

// global data definition of an int variable
func (b *Builder) Data(def DataDef, typ Type, val Value) Var {
	b.dataPrelude(def)

	b.Buf.WriteString("{")
	b.Buf.WriteString(typ.IL())
	b.Buf.WriteString(" ")
	b.Buf.WriteString(val.IL())
	b.Buf.WriteString("}\n")

	return Var{Global: true, Name: def.VarName}
}

func (b *Builder) StrData(def DataDef, val string) Var {
	b.dataPrelude(def)

	b.Buf.WriteString("{")
	b.Buf.WriteString(Byte.IL())
	b.Buf.WriteString(" ")
	b.Buf.WriteString(strconv.Quote(val))
	b.Buf.WriteString(", b 0") // null terminate
	b.Buf.WriteString("}\n")

	return Var{Global: true, Name: def.VarName}
}

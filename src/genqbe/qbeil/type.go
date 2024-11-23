package qbeil

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/horriblename/typee/src/assert"
)

//go-sumtype:decl Type AggregateType

type Type interface {
	typ()
	IL() string
}

type AggregateType interface {
	Type
	Define() string
}

type BaseType int
type StructType struct {
	Align   int // 0 means default: maximum alignment of children
	Name    string
	Layouts map[string]FieldLayout
	Fields  []Type
}
type UnionType struct {
	Name     string
	Align    int
	Size     int
	Variants []Type
}

type FieldLayout struct {
	Type
	Offset int
}

const (
	Word   BaseType = iota // 32-bit int
	Long                   // 64-bit int
	Single                 // 32-bit float
	Double                 // 64-bit float
)

func (BaseType) typ()   {}
func (StructType) typ() {}
func (UnionType) typ()  {}

func (t BaseType) IL() string {
	switch t {
	case Word:
		return "w"
	case Long:
		return "l"
	case Single:
		return "s"
	case Double:
		return "d"
	}

	panic("unreachable")
}
func (t StructType) IL() string {
	return ":" + t.Name
}
func (t UnionType) IL() string {
	return ":" + t.Name
}

func (t StructType) Define() string {
	var b strings.Builder
	_, err := b.WriteString("type :")
	assert.Ok(err)

	_, err = b.WriteString(t.Name)
	assert.Ok(err)

	_, err = b.WriteString(" = ")
	assert.Ok(err)

	if t.Align != 0 {
		_, err = b.WriteString("align " + strconv.Itoa(t.Align))
		assert.Ok(err)
	}

	_, err = b.WriteString("{")
	assert.Ok(err)

	for _, typ := range t.Fields {
		_, err = b.WriteString(typ.IL())
		assert.Ok(err)

		_, err = b.WriteString(", ")
		assert.Ok(err)
	}

	_, err = b.WriteString("}")
	assert.Ok(err)

	return b.String()
}

func (t UnionType) Define() string {
	var b strings.Builder
	_, err := b.WriteString("type :")
	assert.Ok(err)

	_, err = b.WriteString(t.Name)
	assert.Ok(err)

	_, err = b.WriteString(" = ")
	assert.Ok(err)

	if t.Align != 0 {
		_, err = b.WriteString("align " + strconv.Itoa(t.Align))
		assert.Ok(err)
	}

	_, err = b.WriteString("{")
	assert.Ok(err)

	for i, typ := range t.Variants {
		if i != 0 {
			b.WriteByte(' ')
		}

		b.WriteString("{ ")
		_, err = b.WriteString(typ.IL())
		assert.Ok(err)

		_, err = b.WriteString(" }")
		assert.Ok(err)
	}

	_, err = b.WriteString("}")
	assert.Ok(err)

	return b.String()
}

func NewUnionType(name string, align int, defaultAlign int, variants []Type) UnionType {
	maxAlign := 0
	maxBits := 0
	for _, v := range variants {
		bits, align := SizeOf(defaultAlign, v)
		maxBits = max(bits, maxBits)
		maxAlign = max(align, maxAlign)
	}

	if align == 0 {
		align = maxAlign
	}

	return UnionType{
		Name:     name,
		Align:    align,
		Size:     maxBits / 8,
		Variants: variants,
	}
}

func SizeOf(defaultAlign int, t Type) (bits int, align int) {
	switch t := t.(type) {
	case BaseType:
		switch t {
		case Word:
			return 32, defaultAlign
		case Long:
			return 64, defaultAlign
		case Single:
			return 32, defaultAlign
		case Double:
			return 64, defaultAlign
		default:
			panic(fmt.Sprintf("unexpected BaseType: %#v", t))
		}

	case StructType:
		// TODO: calculate size on init
		bits := 0
		align := 0
		for _, field := range t.Fields {
			// TODO: actually handle align
			fs, fa := SizeOf(defaultAlign, field)
			align = max(align, fa)
			bits += fs
		}
		return bits, align

	case UnionType:
		return t.Size, t.Align

	default:
		panic(fmt.Sprintf("unexpected Type: %#v", t))
	}
}

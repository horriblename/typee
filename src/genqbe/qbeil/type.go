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

func (BaseType) typ()   {}
func (ExtType) typ()    {}
func (StructType) typ() {}
func (UnionType) typ()  {}

type RepeatType struct {
	Type Type

	// at least 1, 0 will be interpreted as 1
	Count int
}

type AggregateType interface {
	Type
	ABIType
	Define() string
}

// used for function arg and return types
type ABIType interface {
	abi()
	IL() string
}

type BaseType int

const (
	Word   BaseType = iota // 32-bit int
	Long                   // 64-bit int
	Single                 // 32-bit float
	Double                 // 64-bit float
)

type ExtType int

const (
	Byte     ExtType = iota // 8-bit
	HalfWord                // 16-bit
)

type SubWordType int

const (
	SignedByte   SubWordType = iota // 8-bit
	UnsignedByte                    // 8-bit
	SignedHalf                      // 16-bit
	UnsignedHalf                    // 16-bit
)

type StructType struct {
	Align   int // 0 means default: maximum alignment of children
	Name    string
	Layouts map[string]FieldLayout
	Fields  []RepeatType
}
type UnionType struct {
	Name     string
	Align    int
	Size     int
	Variants []Type
}

type FieldLayout struct {
	Type
	OffsetBits int
}

func (StructType) abi()  {}
func (UnionType) abi()   {}
func (SubWordType) abi() {}
func (BaseType) abi()    {}

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
func (t ExtType) IL() string {
	switch t {
	case Byte:
		return "b"
	case HalfWord:
		return "h"
	}

	panic("unreachable")
}
func (t StructType) IL() string {
	return ":" + t.Name
}
func (t UnionType) IL() string {
	return ":" + t.Name
}

func (t SubWordType) IL() string {
	switch t {
	case SignedByte:
		return "sb"
	case SignedHalf:
		return "sh"
	case UnsignedByte:
		return "ub"
	case UnsignedHalf:
		return "uh"
	default:
		panic(fmt.Sprintf("unexpected qbeil.SubWordType: %#v", t))
	}
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
		_, err = b.WriteString(typ.Type.IL())
		assert.Ok(err)

		if typ.Count > 1 {
			b.WriteString(" ")
			b.WriteString(strconv.Itoa(typ.Count))
		}

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

func SingleType(t Type) RepeatType {
	return RepeatType{t, 1}
}

func NewUnionType(name string, alignBits int, defaultAlign int, variants []Type) UnionType {
	maxAlign := 0
	maxBits := 0
	for _, v := range variants {
		var bits int
		bits, alignBits = SizeOf(defaultAlign, v)
		maxBits = max(bits, maxBits)
		maxAlign = max(alignBits, maxAlign)
	}

	if alignBits == 0 {
		alignBits = maxAlign
	}

	return UnionType{
		Name:     name,
		Align:    alignBits / 8,
		Size:     maxBits / 8,
		Variants: variants,
	}
}

func SizeOf(defaultAlignBits int, t Type) (bits int, alignBits int) {
	switch t := t.(type) {
	case BaseType:
		switch t {
		case Word:
			return 32, defaultAlignBits
		case Long:
			return 64, defaultAlignBits
		case Single:
			return 32, defaultAlignBits
		case Double:
			return 64, defaultAlignBits
		default:
			panic(fmt.Sprintf("unexpected BaseType: %#v", t))
		}

	case StructType:
		// TODO: calculate size on init
		bits := 0
		align := 0
		for _, field := range t.Fields {
			// TODO: actually handle align
			fs, fa := SizeOf(defaultAlignBits, field.Type)
			fs *= min(1, field.Count)
			align = max(align, fa)
			bits += fs
		}
		return bits, align

	case UnionType:
		return t.Size, t.Align

	case ExtType:
		switch t {
		case Byte:
			return 8, defaultAlignBits
		case HalfWord:
			return 16, defaultAlignBits
		default:
			panic(fmt.Sprintf("unexpected qbeil.ExtType: %#v", t))
		}
	default:
		panic(fmt.Sprintf("unexpected Type: %v", t))
	}
}

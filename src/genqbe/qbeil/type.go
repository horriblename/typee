package qbeil

import (
	"strconv"
	"strings"

	"github.com/horriblename/typee/src/assert"
)

type Type interface {
	typ()
	IL() string
}

type BaseType int
type StructType struct {
	Align  int // 0 means default: maximum alignment of children
	Name   string
	Fields []Type
}

const (
	Word   BaseType = iota // 32-bit int
	Long                   // 64-bit int
	Single                 // 32-bit float
	Double                 // 64-bit float
)

func (BaseType) typ()   {}
func (StructType) typ() {}

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
	}

	_, err = b.WriteString("}")
	assert.Ok(err)

	return b.String()
}

// package qbeil provides functions to write QBE IL output
package qbeil

import (
	"fmt"
	"io"
	"strings"
)

const indentSym string = "\t"

type Builder struct {
	Writer    io.Writer
	indentLvl int
}

type TypedVar struct {
	typ  Type
	name string
}

func (v TypedVar) String() string {
	return fmt.Sprintf("%s %s", v.typ, v.name)
}

func NewTypedVar(typ Type, name string) TypedVar {
	return TypedVar{typ: typ, name: name}
}

func (b *Builder) indented(l []byte) error {
	_, err := b.Writer.Write([]byte(strings.Repeat(indentSym, b.indentLvl)))
	if err != nil {
		return err
	}

	_, err = b.Writer.Write(l)
	if err != nil {
		return err
	}

	return nil
}

func (b *Builder) Func(ret Type, name string, args []TypedVar) error {
	err := b.indented([]byte(fmt.Sprintf("function %s %s(", ret, name)))
	if err != nil {
		return err
	}

	if len(args) > 0 {
		_, err = b.Writer.Write([]byte(args[0].String()))
		if err != nil {
			return err
		}

		for _, arg := range args[1:] {
			_, err := b.Writer.Write([]byte(", "))
			if err != nil {
				return err
			}

			_, err = b.Writer.Write([]byte(arg.String()))
			if err != nil {
				return err
			}
		}
	}

	b.Writer.Write([]byte(") {"))
	b.indentLvl++

	return nil
}

func (b *Builder) EndFunc() {
	b.indentLvl--
	b.Writer.Write([]byte("}"))
}

// package qbeil provides functions to write QBE IL output
package qbeil

import (
	"fmt"
	"io"
	"strings"

	"github.com/horriblename/typee/src/fun"
)

const indentSym string = "\t"

type Builder struct {
	Writer    io.Writer
	indentLvl int
	tempID    int
}

type TypedVar struct {
	typ  Type
	name Var
}

func (v TypedVar) IL() string {
	return fmt.Sprintf("%s %s", v.typ.IL(), v.name.IL())
}

func NewTypedVar(typ Type, name Var) TypedVar {
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

func (b *Builder) Func(linkage Linkage, ret *Type, name string, args []TypedVar) error {
	linkageStr := linkage.String()
	if linkageStr != "" {
		linkageStr += " "
	}

	returnType := ""
	if ret != nil {
		returnType = (*ret).IL() + " "
	}

	err := b.indented([]byte(fmt.Sprintf("%sfunction %s%s(", linkageStr, returnType, name)))
	if err != nil {
		return err
	}

	if len(args) > 0 {
		_, err = b.Writer.Write([]byte(args[0].IL()))
		if err != nil {
			return err
		}

		for _, arg := range args[1:] {
			_, err := b.Writer.Write([]byte(", "))
			if err != nil {
				return err
			}

			_, err = b.Writer.Write([]byte(arg.IL()))
			if err != nil {
				return err
			}
		}
	}

	b.Writer.Write([]byte(") {\n"))
	b.Label("start")
	b.indentLvl++

	return nil
}

func (b *Builder) EndFunc() {
	b.indentLvl--
	b.indented([]byte("}\n"))
}

func (b *Builder) Label(name string) {
	b.indented([]byte(fmt.Sprint("@", name, "\n")))
}

func (b *Builder) Ret(val Value) {
	b.indented([]byte(fmt.Sprintf("ret %s\n", val.IL())))
}

func (b *Builder) Arithmetic(target string, ret Type, op string, left Value, right Value) {
	retStr := ""
	if ret != nil {
		retStr = "=" + ret.IL()
	}
	b.indented([]byte(fmt.Sprintf("%s %s %s %s %s\n", target, retStr, op, left.IL(), right.IL())))
}

func (b *Builder) TempVar() Var {
	b.tempID++
	return Var{Global: false, Name: fmt.Sprintf("_tmp_%d", b.tempID)}
}

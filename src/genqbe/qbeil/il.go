// package qbeil provides functions to write QBE IL output
package qbeil

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/horriblename/typee/src/fun"
)

var indentSym = []byte{'\t'}

type Builder struct {
	// where we write to most of the time
	Buf bytes.Buffer

	// not currently used. Allows user to add "prefix" code before writing
	// our result IL from Buf
	OutFile   io.Writer
	indentLvl int

	// ID generator
	tempID int
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
	b.Buf.Write(bytes.Repeat(indentSym, b.indentLvl))
	b.Buf.Write(l)

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
		b.Buf.Write([]byte(args[0].IL()))

		for _, arg := range args[1:] {
			b.Buf.Write([]byte(", "))
			b.Buf.Write([]byte(arg.IL()))
		}
	}

	b.Buf.Write([]byte(") {\n"))
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
func (b *Builder) Arithmetic(target string, ret Type, op string, args ...Value) {
	retStr := ""
	if ret != nil {
		retStr = "=" + ret.IL()
	}

	argsStr := fun.Map(args, func(arg Value) string { return arg.IL() })
	argStr := strings.Join(argsStr, ", ")

	b.indented([]byte(fmt.Sprintf("%s %s %s %s\n", target, retStr, op, argStr)))
}

type DataDef struct {
	Linkage Linkage
	VarName string
	Align   int // 0 for auto
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

func (b *Builder) dataPrelude(def DataDef) {
	linkage := def.Linkage.String()
	if linkage != "" {
		linkage += " "
	}

	b.Buf.WriteString(linkage)
	b.Buf.WriteString("data $")
	b.Buf.WriteString(def.VarName)
	b.Buf.WriteString(" = ")
	if def.Align != 0 {
		b.Buf.WriteString(" align ")
		b.Buf.WriteString(strconv.Itoa(def.Align))
	}
}

func (b *Builder) Command(op string, args ...Value) {
	argStrs := fun.Map(args, func(arg Value) string { return arg.IL() })
	argStr := strings.Join(argStrs, ", ")

	b.indented([]byte(fmt.Sprintf("%s %s\n", op, argStr)))
}

func (b *Builder) Call(target *Var, typ Type, name Var, args []TypedValue) {
	argsStr := strings.Join(fun.Map(args, func(v TypedValue) string {
		return fmt.Sprint(v.Type.IL(), " ", v.Value.IL())
	}), ", ")
	if target != nil {
		b.indented(
			[]byte(fmt.Sprintf(
				"%s =%s call %s (%s)\n",
				target.IL(), typ.IL(), name.IL(), argsStr)),
		)
	} else {
		b.indented([]byte(fmt.Sprintf("call %s (%s)\n", name.IL(), argsStr)))
	}
}

func (b *Builder) TempVar(global bool) Var {
	b.tempID++
	return Var{Global: global, Name: fmt.Sprintf("_tmp_%d", b.tempID)}
}

package gir

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/linuxdeepin/go-gir/generator/gi"
)

type Generator struct {
	namespace    string
	classInScope []string
	goBindings   bytes.Buffer
}

func Gen(lib string, version string) ([]byte, error) {
	g := Generator{lib, []string{}, bytes.Buffer{}}
	err := g.Gen(lib, version)
	if err != nil {
		return nil, err
	}

	return g.goBindings.Bytes(), nil
}

func (self *Generator) Gen(lib string, version string) error {
	repo := gi.DefaultRepository()
	_, err := repo.Require(lib, version, 0)
	if err != nil {
		return err
	}

	for i, n := 0, repo.NumInfo(self.namespace); i < n; i++ {
		self.process_base_info(repo.Info(self.namespace, i))
	}

	return nil
}

func (self *Generator) process_base_info(bi *gi.BaseInfo) {
	switch bi.Type() {
	case gi.INFO_TYPE_UNION:
		self.processUnionInfo(gi.ToUnionInfo(bi))
	case gi.INFO_TYPE_STRUCT:
		self.processStructInfo(gi.ToStructInfo(bi))
	case gi.INFO_TYPE_ENUM, gi.INFO_TYPE_FLAGS:
		self.processEnumInfo(gi.ToEnumInfo(bi))
	case gi.INFO_TYPE_CONSTANT:
		self.processConstantInfo(gi.ToConstantInfo(bi))
	case gi.INFO_TYPE_CALLBACK:
		self.processCallbackInfo(gi.ToCallableInfo(bi))
	case gi.INFO_TYPE_FUNCTION:
		self.processFunctionInfo(gi.ToFunctionInfo(bi))
	case gi.INFO_TYPE_INTERFACE:
		self.processInterfaceInfo(gi.ToInterfaceInfo(bi))
	case gi.INFO_TYPE_OBJECT:
		self.processObjectInfo(gi.ToObjectInfo(bi))
	}
}

func (self *Generator) processUnionInfo(ui *gi.UnionInfo) {
	p := printerTo(&self.goBindings)

	name := ui.Name()
	p("(union %s {\n", name)
	p("  _data [%d]U8\n", ui.Size())
	self.classInScope = append(self.classInScope, name)
	defer func() { popDelete(&self.classInScope) }()

	for i, n := 0, ui.NumMethod(); i < n; i++ {
		meth := ui.Method(i)
		self.processFunctionInfo(meth)
	}

	p("})\n")
}

func (self *Generator) processStructInfo(si *gi.StructInfo) {
	p := printerTo(&self.goBindings)

	name := si.Name()
	size := si.Size()

	self.classInScope = append(self.classInScope, name)
	defer popDelete(&self.classInScope)

	if si.IsGTypeStruct() {
		return
	}
	if strings.HasSuffix(name, "Private") {
		return
	}

	// fullnm := si.Namespace() + "." + name
	// if config.is_disguised(fullnm) {
	// 	size = -1
	// }

	// if !config.is_blacklisted("structdefs", name) {
	switch size {
	case -1:
		p("(type %s opaque)\n", name)
	case 0:
		p("type %s {})\n", name)
	default:
		p("(type %s {\n", name)
		offset := 0
		for i, n := 0, si.NumField(); i < n; i++ {
			field := si.Field(i)
			fo := field.Offset()
			ft := field.Type()
			nm := field.Name()
			if fo != offset {
				pad := fo - offset
				p("\t_ [%d]byte\n", pad)
				offset += pad
			}
			// if type_needs_wrapper(ft) {
			// 	p("\t%s0 %s\n", nm, cgo_type(ft, type_exact))
			// } else {
			p("\t%s: %s\n", snake_case_to_camelCase(nm),
				horType(ft, typeConfig{typeExact, field.Namespace()}))
			// }
			offset += typeSize(ft, typeExact)
		}
		if size != offset {
			p("\t_: [%d]byte\n", size-offset)
		}
		p("})\n")
		//printf("type %s struct { data [%d]byte }\n", name, size)
	}

	for i, n := 0, si.NumMethod(); i < n; i++ {
		meth := si.Method(i)
		// if config.is_method_blacklisted(name, meth.Name()) {
		// 	continue
		// }

		self.processFunctionInfo(meth)
	}
}
func (self *Generator) processEnumInfo(ei *gi.EnumInfo) {
	p := printerTo(&self.goBindings)

	p("(enum %s {\n", ei.Name())
	for i, n := 0, ei.NumValue(); i < n; i++ {
		val := ei.Value(i)
		p("  %s:%d\n", snake_case_to_PascalCase(val.Name()), val.Value())
	}
	p("})\n")
}
func (self *Generator) processConstantInfo(ci *gi.ConstantInfo) {
	p := printerTo(&self.goBindings)

	name := ci.Name()
	if self.namespace == "Gdk" && strings.HasPrefix(name, "KEY_") {
		// KEY_ constants maybe deserve special treatment?
		p("(const Key_%s %s)\n", name[4:], ci.Value())
		return
	}
	p("(const %s %#v)\n", snake_case_to_PascalCase(name), ci.Value())
}
func (self *Generator) processCallbackInfo(ci *gi.CallableInfo) {
	// p := printerTo(&this.goBindings)
	//
	// var args []*gi.ArgInfo
	// for i, n := 0, ci.NumArg(); i < n; i++ {
	// 	arg := ci.Arg(i)
	// 	args = append(args, arg)
	// }
	//
	// userdata := -1
	// for i, arg := range args {
	// 	if arg.Closure() != -1 {
	// 		userdata = i
	// 		break
	// 	}
	//
	// 	// treat any void* as userdata o_O
	// 	t := arg.Type()
	// 	if t.Tag() == gi.TYPE_TAG_VOID && t.IsPointer() {
	// 		userdata = i
	// 		break
	// 	}
	// }
	//
	// if userdata == -1 {
	// 	p("# blacklisted (no userdata): ")
	// }
	//
	// name := ci.Name()
	// p("(: %s (fn [", name)
	// for i, ri, n := 0, 0, len(args); i < n; i++ {
	// 	if i == userdata {
	// 		continue
	// 	}
	//
	// 	// I use here TypeReturn because for closures it's inverted
	// 	// C code calls closure and it has to be concrete and Go code
	// 	// returns stuff to C (like calling a C function)
	// }
	// p("]")
	//
	// p("))")
}
func (self *Generator) processFunctionInfo(fi *gi.FunctionInfo) {
	p := printerTo(&self.goBindings)

	// var fullName string
	flags := fi.Flags()
	name := fi.Name()

	if (flags&gi.FUNCTION_IS_METHOD != 0) && len(self.classInScope) == 0 {
		panic(fmt.Sprintf("tried processing a method %s but no current class", name))
	}
	container := fi.Container()
	isValidMethod := flags&gi.FUNCTION_IS_METHOD != 0 && len(self.classInScope) != 0
	fb := newFunctionBuilder(fi)

	p("(def ")

	switch {
	case flags&gi.FUNCTION_IS_CONSTRUCTOR != 0:
		name = "init"
	case isValidMethod:
		name = snake_case_to_camelCase(name)
	default:
		name = snake_case_to_camelCase(name)
	}
	// fullName += name
	p("%s (", name)
	if isValidMethod {
		p("Self")
	}
	for i, arg := range fb.args {
		if i != 0 || isValidMethod {
			p(" ")
		}
		p("%s", horType(arg.typeInfo, typeConfig{typeNone, self.namespace}))
	}

	if len(fb.args) > 0 || isValidMethod {
		p(" ")
	}

	switch len(fb.rets) {
	case 0:
		p("{}")

	case 1:
		if flags&gi.FUNCTION_IS_CONSTRUCTOR != 0 {
			p("%s", container.Name())
		}
		p("%s", horType(fb.rets[0].typeInfo, typeConfig{typeNone, self.namespace}))

	default:
		p("(Tuple")
		for _, ret := range fb.rets {
			p(" %s", horType(ret.typeInfo, typeConfig{typeNone, self.namespace}))
		}
		p(")")
	}

	p(") [")
	if isValidMethod {
		p("self")
	}
	for i, arg := range fb.args {
		if i != 0 || isValidMethod {
			p(" ")
		}
		p("%s", snake_case_to_camelCase(arg.argInfo.Name()))
	}
	p("])\n")

}

func (self *Generator) processInterfaceInfo(ii *gi.InterfaceInfo) {
	p := printerTo(&self.goBindings)

	name := ii.Name()
	self.classInScope = append(self.classInScope, name)
	defer func() { popDelete(&self.classInScope) }()

	p("(interface %s {\n", name)

	for i, n := 0, ii.NumMethod(); i < n; i++ {
		if i != 0 {
			p(", ")
		}
		meth := ii.Method(i)
		self.processFunctionInfo(meth)
	}

	p("})\n")
}

func (self *Generator) processObjectInfo(oi *gi.ObjectInfo) {
	// TODO: are there nested class?
	self.classInScope = append(self.classInScope, oi.Name())
	defer func() { popDelete(&self.classInScope) }()

	p := printerTo(&self.goBindings)

	p("(class %s (", snake_case_to_PascalCase(oi.Name()))

	for i, n := 0, oi.NumInterface(); i < n; i++ {
		ii := oi.Interface(i)
		name := ii.Name()
		ns := ii.Namespace()
		if i != 0 {
			p(" ")
		}
		if ns != self.namespace {
			p("%s.", snake_case_to_PascalCase(ns))
		}
		p("%s", snake_case_to_PascalCase(name))
	}

	p(") {\n")

	for i, n := 0, oi.NumMethod(); i < n; i++ {
		if i != 0 {
			p(", ")
		}
		meth := oi.Method(i)
		self.processFunctionInfo(meth)
	}
	p("})\n")
}

func printerTo(w io.Writer) func(format string, args ...any) {
	return func(format string, args ...any) {
		_, err := fmt.Fprintf(w, format, args...)
		if err != nil {
			panic("TODO: unhandled: " + err.Error())
		}
	}
}

func popDelete[T any](xs *[]T) {
	var zero T
	(*xs)[len(*xs)-1] = zero
	*xs = (*xs)[:len(*xs)-1]
}

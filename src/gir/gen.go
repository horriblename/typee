package gir

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/horriblename/typee/src/parse"
	"github.com/linuxdeepin/go-gir/generator/gi"
)

type Generator struct {
	namespace    string
	classInScope []string
	goBindings   bytes.Buffer
}

func (self *Generator) Gen(lib string, version string) (parse.Expr, error) {
	repo := gi.DefaultRepository()
	_, err := repo.Require(lib, version, 0)
	if err != nil {
		return nil, err
	}

	fmt.Printf("loaded ns: %+v\n", repo.LoadedNamespaces())
	for i, n := 0, repo.NumInfo(self.namespace); i < n; i++ {
		self.process_base_info(repo.Info(self.namespace, i))
	}

	return nil, nil
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

func (self *Generator) processStructInfo(*gi.StructInfo) {}
func (self *Generator) processEnumInfo(ei *gi.EnumInfo) {
	p := printerTo(&self.goBindings)

	p("(enum %s {\n", ei.Name())
	for i, n := 0, ei.NumValue(); i < n; i++ {
		val := ei.Value(i)
		p("  %s:%d,\n", snake_case_to_PascalCase(val.Name()), val.Value())
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
		p("%s", horType(arg.typeInfo, typeNone))
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
	fmt.Printf("%+s", self.classInScope)
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

func horType(ti *gi.TypeInfo, flags typeFlags) string {
	var out bytes.Buffer

	switch tag := ti.Tag(); tag {
	case gi.TYPE_TAG_VOID:
		if ti.IsPointer() {
			out.WriteString("opaque")
			break
		}
		panic("Non-pointer void type is not supported")
	case gi.TYPE_TAG_UTF8, gi.TYPE_TAG_FILENAME:
		if flags&typeExact != 0 {
			out.WriteString("opaque")
		} else {
			out.WriteString("Str")
		}
	case gi.TYPE_TAG_ARRAY:
		size := ti.ArrayFixedSize()
		out.WriteString(horType(ti.ParamType(0), flags))
		if size != -1 {
			fmt.Fprintf(&out, "[%d]", size)
		} else {
			out.WriteString("[]")
		}
	case gi.TYPE_TAG_GLIST:
		out.WriteString(horType(ti.ParamType(0), flags))
		out.WriteString("[]")
	case gi.TYPE_TAG_GSLIST:
		out.WriteString(horType(ti.ParamType(0), flags))
		out.WriteString("[]")
	case gi.TYPE_TAG_GHASH:
		// out.WriteString("map[")
		// out.WriteString(horType(ti.ParamType(0), flags))
		// out.WriteString("]")
		// out.WriteString(horType(ti.ParamType(1), flags))
	case gi.TYPE_TAG_ERROR:
		// not used?
		// out.WriteString("error")
	case gi.TYPE_TAG_INTERFACE:
		// TODO
		if ti.IsPointer() {
			flags |= typePointer
		}
		out.WriteString(horTypeForInterface(ti.Interface(), flags))
	default:
		// TODO
		if ti.IsPointer() {
			flags |= typePointer
		}
		out.WriteString(horTypeForTag(tag, flags))
	}
	return out.String()
}

func horTypeForTag(tag gi.TypeTag, flags typeFlags) string {
	var out bytes.Buffer
	p := printerTo(&out)

	if flags&typePointer != 0 {
		p("*")
	}

	if flags&typeExact != 0 {
		switch tag {
		case gi.TYPE_TAG_BOOLEAN:
			p("int32") // sadly
		case gi.TYPE_TAG_INT8:
			p("int8")
		case gi.TYPE_TAG_UINT8:
			p("uint8")
		case gi.TYPE_TAG_INT16:
			p("int16")
		case gi.TYPE_TAG_UINT16:
			p("uint16")
		case gi.TYPE_TAG_INT32:
			p("int32")
		case gi.TYPE_TAG_UINT32:
			p("uint32")
		case gi.TYPE_TAG_INT64:
			p("int64")
		case gi.TYPE_TAG_UINT64:
			p("uint64")
		case gi.TYPE_TAG_FLOAT:
			p("float32")
		case gi.TYPE_TAG_DOUBLE:
			p("float64")
		case gi.TYPE_TAG_GTYPE:
			// if config.namespace != "GObject" {
			// 	p("gobject.Type")
			// } else {
			p("Type")
		case gi.TYPE_TAG_UNICHAR:
			p("U32")
		default:
			panic("unreachable")
		}
	} else {
		switch tag {
		case gi.TYPE_TAG_BOOLEAN:
			p("Bool")
		case gi.TYPE_TAG_INT8:
			p("I8")
		case gi.TYPE_TAG_UINT8:
			p("U8")
		case gi.TYPE_TAG_INT16:
			p("I16")
		case gi.TYPE_TAG_UINT16:
			p("U16")
		case gi.TYPE_TAG_INT32:
			p("I32")
		case gi.TYPE_TAG_UINT32:
			p("U32")
		case gi.TYPE_TAG_INT64:
			p("I64")
		case gi.TYPE_TAG_UINT64:
			p("U64")
		case gi.TYPE_TAG_FLOAT:
			p("F32")
		case gi.TYPE_TAG_DOUBLE:
			p("F64")
		case gi.TYPE_TAG_GTYPE:
			// if config.namespace != "GObject" {
			// 	p("gobject.Type")
			// } else {
			p("Type")
		case gi.TYPE_TAG_UNICHAR:
			p("U32")
		default:
			panic("unreachable")
		}
	}

	return out.String()
}

func horTypeForInterface(bi *gi.BaseInfo, flags typeFlags) string {
	var out bytes.Buffer
	p := printerTo(&out)
	ns := snake_case_to_PascalCase(bi.Namespace())

	if flags&typeListMember != 0 {
		switch bi.Type() {
		case gi.INFO_TYPE_OBJECT, gi.INFO_TYPE_INTERFACE:
			return horTypeForInterface(bi, typePointer|typeReturn)
		default:
			return horTypeForInterface(bi, typeReturn)
		}
	}

	switch t := bi.Type(); t {
	case gi.INFO_TYPE_OBJECT, gi.INFO_TYPE_INTERFACE:
		if flags&typeExact != 0 {
			// exact type for object/interface is always an unsafe.Pointer
			p("opaque")
			break
		}

		if flags&(typeReturn|typeReceiver) != 0 && flags&typePointer != 0 {
			// receivers and return values are actual types,
			// and a pointer most likely
			p("*")
		}
		// TODO:  check namespace
		// prepend foreign types with appropriate namespace
		p("%s.", ns)

		p(bi.Name())

	case gi.INFO_TYPE_CALLBACK:
		if flags&typeExact != 0 {
			p("opaque")
			break
		}
		goto handle_default
	case gi.INFO_TYPE_STRUCT:
		goto handle_default
	default:
		goto handle_default
	}
	return out.String()
handle_default:
	if flags&typePointer != 0 /* && !config.is_disguised(fullnm) */ {
		p("*")
	}
	// TODO: check namespace
	p("%s.", ns)
	p(bi.Name())
	return out.String()
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

package gir

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/linuxdeepin/go-gir/generator/gi"
)

type Generator struct {
	config      Config
	namespace   string
	inStruct    bool
	methodOwner []string
	goBindings  bytes.Buffer
	externs     bytes.Buffer
}

func New(lib string, version string, config Config) ([]byte, error) {
	g := Generator{config, lib, false, []string{}, bytes.Buffer{}, bytes.Buffer{}}
	g.externs.WriteString("\n;; extern declarations\n")
	err := g.Gen(lib, version)
	if err != nil {
		return nil, err
	}

	config.writeExtraTypes(&g.externs)
	g.goBindings.WriteTo(&g.externs)
	return g.externs.Bytes(), nil
}

func (self *Generator) Gen(lib string, version string) error {
	repo := gi.DefaultRepository()
	_, err := repo.Require(lib, version, 0)
	if err != nil {
		return err
	}

	deps := repo.Dependencies(self.namespace)
	for _, dep := range deps {
		i := 0
		for ; i < len(dep); i++ {
			if dep[i] == '-' {
				break
			}
		}

		name := dep[:i]
		fmt.Fprintf(&self.externs, "(import %s)\n", name)
	}
	for i, n := 0, repo.NumInfo(self.namespace); i < n; i++ {
		self.process_base_info(repo.Info(self.namespace, i))
	}

	return nil
}

func (self *Generator) process_base_info(bi *gi.BaseInfo) {
	p := printerTo(&self.goBindings)

	if self.config.is_object_blacklisted(bi) {
		p(";; blacklisted: %s (%s) \n", bi.Name(), bi.Type())
		return
	}

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
	p("(type %s {\n", name)
	p("  _data: [U8 %d]\n", ui.Size())
	self.methodOwner = append(self.methodOwner, name)
	self.inStruct = true
	defer func() { popDelete(&self.methodOwner) }()
	defer func() { self.inStruct = false }()
	p("})\n")

	for i, n := 0, ui.NumMethod(); i < n; i++ {
		meth := ui.Method(i)
		if self.config.is_method_blacklisted(name, meth.Name()) {
			continue
		}
		self.processFunctionInfo(meth)
	}

}

func (self *Generator) processStructInfo(si *gi.StructInfo) {
	p := printerTo(&self.goBindings)

	name := si.Name()
	size := si.Size()

	if self.config.is_blacklisted("structdefs", name) {
		// FIXME: go-gir processes methods even if struct is blacklisted
		p(";; blacklisted: %s (struct)", name)
		return
	}

	self.inStruct = true
	self.methodOwner = append(self.methodOwner, name)
	defer popDelete(&self.methodOwner)
	defer func() { self.inStruct = false }()

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
		p("(type %s Opaque)\n", name)
	case 0:
		p("(type %s {})\n", name)
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
				p("\t_%s: [U8 %d],", nm, pad)
				offset += pad
			}
			// if type_needs_wrapper(ft) {
			// 	p("\t%s0 %s\n", nm, cgo_type(ft, type_exact))
			// } else {
			p("\t%s: %s", sanitize(snake_case_to_camelCase(nm)),
				horType(ft, typeConfig{typeExact, field.Namespace()}))
			// }
			offset += typeSize(ft, typeExact)
			if i != n {
				p(",")
			}
			p("\n")
		}
		if size != offset {
			p("\t_: [U8 %d]\n", size-offset)
		}
		p("})\n")
		//printf("type %s struct { data [%d]byte }\n", name, size)
	}

	for i, n := 0, si.NumMethod(); i < n; i++ {
		meth := si.Method(i)
		if self.config.is_method_blacklisted(name, meth.Name()) {
			continue
		}

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
	var val string
	switch v := ci.Value().(type) {
	case bool:
		if v {
			val = "true"
		} else {
			val = "false"
		}
	case int8, uint8, int16, uint16, int32, uint32, int64, uint64:
		val = fmt.Sprintf("%d", v)
	case float32, float64:
		val = fmt.Sprintf("%f", v)
	case string:
		val = strconv.Quote(v)
	}
	p("(set %s %s)\n", sanitize(CONST_CASE_to_camelCase(name)), val)
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
	if fi.IsDeprecated() {
		p(";; skipped: %s (deprecated function)\n", fi.Name())
		return
	}

	var fullName string
	flags := fi.Flags()
	name := fi.Name()

	if (flags&gi.FUNCTION_IS_METHOD != 0) && len(self.methodOwner) == 0 {
		panic(fmt.Sprintf("tried processing a method %s but no current class", name))
	}
	// container := fi.Container()
	isValidMethod := flags&gi.FUNCTION_IS_METHOD != 0 && len(self.methodOwner) != 0
	fb := newFunctionBuilder(fi)

	if self.inStruct {
		owner := self.methodOwner[len(self.methodOwner)-1]
		fullName = strings.ToLower(owner[:1]) + owner[1:] + "_"
	}

	// wrapper function
	p("(def ")

	name = snake_case_to_camelCase(name)
	fullName += name
	p("%s (", sanitize(fullName))

	// wrapper function type signature

	if isValidMethod {
		if self.inStruct {
			p("(Ref %s)", (self.methodOwner[len(self.methodOwner)-1]))
		} else {
			p("Self")
		}
	}
	for i, arg := range fb.args {
		if i != 0 || isValidMethod {
			p(" ")
		}
		flags := typeNone
		p("%s", horType(arg.typeInfo, typeConfig{flags, self.namespace}))
	}

	if len(fb.args) > 0 || isValidMethod || self.inStruct /* methods in structs aren't gi.FUNCTION_IS_METHOD */ {
		p(" ")
	}

	// wrapper function return type

	switch len(fb.rets) {
	case 0:
		p("{}")

	case 1:
		// // why tf did I use container???
		// if flags&gi.FUNCTION_IS_CONSTRUCTOR != 0 {
		// 	p("(Ref %s)", container.Name())
		// } else {
		p("%s", horType(fb.rets[0].typeInfo, typeConfig{typeNone, self.namespace}))
		// }

	default:
		p("{")
		for i, ret := range fb.rets {
			if ret.index == -2 {
				p("err: Error, ")
				continue
			}

			if ret.index >= 0 {
				p("%s: %s, ", ret.argInfo.Name(), horType(ret.typeInfo, typeConfig{typeNone, self.namespace}))
			} else {
				p("_%d: %s, ", i, horType(ret.typeInfo, typeConfig{typeNone, self.namespace}))
			}
		}
		p("}")
	}

	// wrapper function args

	p(") [")
	if isValidMethod {
		if self.inStruct {
			p("self_")
		} else {
			p("self")
		}
	}
	for i, arg := range fb.args {
		if i != 0 || isValidMethod {
			p(" ")
		}
		p("%s", sanitize(snake_case_to_camelCase(arg.argInfo.Name())))
	}

	// wrapper function body

	p("] ")

	p("(let [\n")
	for _, arg := range fb.orig_args {
		if arg.Direction() == gi.DIRECTION_OUT || arg.Direction() == gi.DIRECTION_INOUT {
			p("    %s (stackAlloc)\n", sanitize(snake_case_to_camelCase(arg.Name())))
		}
	}
	// call to extern function
	p("    ret (%s", fi.Symbol())

	if isValidMethod {
		if self.inStruct {
			p(" self_")
		} else {
			p(" self")
		}
	}
	for _, arg := range fb.orig_args {
		p(" %s", sanitize(snake_case_to_camelCase(arg.Name())))
	}

	p(")\n  ]\n    ")

	// wrapper return value

	retValue := func(ret funcBuilderArg) string {
		if ret.index == -1 { // main return value
			return "ret"
		} else if ret.index == -2 {
			// error value
			panic("unimpl")
		} else {
			val := sanitize(snake_case_to_camelCase(ret.argInfo.Name()))
			if ret.argInfo.Direction() != gi.DIRECTION_IN {
				return fmt.Sprintf("(deref %s)", val)
			}
			return val
		}
	}

	if len(fb.rets) == 0 {
		p("{}")
	} else if len(fb.rets) == 1 {
		p("%s", retValue(fb.rets[0]))
	} else {
		p("{\n")
		for i, ret := range fb.rets {
			if ret.index >= 0 {
				p("    %s: %s,\n", ret.argInfo.Name(), retValue(ret))
			} else {
				p("    _%d: %s,\n", i, retValue(ret))
			}
		}
		p("}")
	}

	p("))\n")

	// extern declaration

	extern := printerTo(&self.externs)
	extern("(extern def %s (", fi.Symbol())

	// extern type signature

	if isValidMethod {
		if self.inStruct {
			extern("(Ref %s) ", self.methodOwner[len(self.methodOwner)-1])
		} else {
			extern("%s ", self.methodOwner[len(self.methodOwner)-1])
		}
	}

	for _, arg := range fb.orig_args {
		if arg.Direction() == gi.DIRECTION_OUT || arg.Direction() == gi.DIRECTION_INOUT {
			// note: wrap Ref outside of horType to allow potential (Ref Opaque), and
			// maybe (Ref (Ref _)) types
			extern("(Ref %s) ", horType(arg.Type(), typeConfig{typeNone, self.namespace}))
		} else {
			extern("%s ", horType(arg.Type(), typeConfig{typeNone, self.namespace}))
		}
	}
	if fi.ReturnType().Tag() == gi.TYPE_TAG_VOID && !fi.ReturnType().IsPointer() {
		// non-pointer void return
		extern("{}) [")
	} else {
		// // why tf was I using container name
		// if flags&gi.FUNCTION_IS_CONSTRUCTOR != 0 {
		// 	extern("(Ref %s)) [", container.Name())
		// } else {
		extern("%s) [", horType(fi.ReturnType(), typeConfig{typeNone, self.namespace}))
		// }
	}

	// extern arguments

	if isValidMethod {
		extern("self_ ")
	}

	for i, arg := range fb.orig_args {
		if i != 0 {
			extern(" ")
		}
		extern("%s", sanitize(snake_case_to_camelCase(arg.Name())))
	}
	extern("])\n")
}

func (self *Generator) processInterfaceInfo(ii *gi.InterfaceInfo) {
	p := printerTo(&self.goBindings)

	name := ii.Name()
	self.inStruct = false
	self.methodOwner = append(self.methodOwner, name)
	defer func() { popDelete(&self.methodOwner) }()

	p("(interface %s {\n", name)

	for i, n := 0, ii.NumMethod(); i < n; i++ {
		meth := ii.Method(i)
		if self.config.is_method_blacklisted(name, meth.Name()) {
			p(";; blacklisted: %s.%s (method)\n", name, meth.Name())
			continue
		}
		if meth.IsDeprecated() {
			p(";; blacklisted: %s.%s (deprecated method)\n", name, meth.Name())
			continue
		}
		self.processFunctionInfo(meth)
		p(", ")
	}

	p("})\n")
}

func (self *Generator) processObjectInfo(oi *gi.ObjectInfo) {
	// TODO: are there nested class?
	self.inStruct = false
	self.methodOwner = append(self.methodOwner, oi.Name())
	defer func() { popDelete(&self.methodOwner) }()

	name := oi.Name()
	p := printerTo(&self.goBindings)

	p("(class %s (", snake_case_to_PascalCase(name))

	if self.namespace == "GObject" && oi.Name() == "Object" {
		p("{}")
	}

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
		meth := oi.Method(i)
		if self.config.is_method_blacklisted(name, meth.Name()) {
			p(";; blacklisted: %s.%s (method)\n", name, meth.Name())
			continue
		}
		if meth.IsDeprecated() {
			p(";; blacklisted: %s.%s (deprecated method)\n", name, meth.Name())
			continue
		}
		self.processFunctionInfo(meth)
		p(", ")
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

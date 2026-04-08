package gir

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

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

func Gen(lib string, version string, config Config) ([]byte, error) {
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
		name, _, _ := strings.Cut(dep, "-")
		fmt.Fprintf(&self.externs, "(import %s)\n", name)
	}
	for i, n := 0, repo.NumInfo(self.namespace); i < n; i++ {
		self.process_base_info(repo.Info(self.namespace, i))
	}

	return nil
}

const rw_r__r__ os.FileMode = 0644

func GenRecursively(lib string, version string, configDir string, destDir string) error {
	repo := gi.DefaultRepository()
	_, err := repo.Require(lib, version, 0)
	if err != nil {
		return err
	}

	ctx := recursiveCtx{
		repo:             repo,
		configDir:        configDir,
		destDir:          destDir,
		generatedVersion: map[string]string{},
	}
	return genRecursively(&ctx, lib, version)
}

type recursiveCtx struct {
	repo             *gi.Repository
	configDir        string
	destDir          string
	generatedVersion map[string]string
}

func genRecursively(ctx *recursiveCtx, lib string, version string) error {
	// TODO: check name conflict of different-versioned dependencies?
	deps := ctx.repo.Dependencies(lib)
	for _, dep := range deps {
		name, ver, _ := strings.Cut(dep, "-")
		if err := genRecursively(ctx, name, ver); err != nil {
			return err
		}
	}

	if hasVer, ok := ctx.generatedVersion[lib]; ok {
		if hasVer != version {
			return fmt.Errorf(
				"%s requires two different versions: %s and %s",
				lib, version, hasVer,
			)
		}
		return nil
	}
	ctx.generatedVersion[lib] = version

	wrap := func(e error) error {
		if e == nil {
			return e
		}
		return fmt.Errorf("generating %s.hor: %w", lib, e)
	}

	configPath := path.Join(ctx.configDir, lib, "config.json")
	destPath := path.Join(ctx.destDir, lib+".hor")

	file, err := os.Open(configPath)
	var config Config
	if err == nil {
		defer file.Close()
		config, err = ParseConfig(file)
		if err != nil {
			return wrap(err)
		}
		fmt.Fprintln(os.Stderr, "Generating", destPath, "with config file", configPath)
	} else if errors.Is(err, os.ErrNotExist) {
		config = Config{
			Blacklist:       map[string]map[string]bool{},
			Whitelist:       map[string]map[string]bool{},
			MethodBlacklist: map[string]map[string]bool{},
			MethodWhitelist: map[string]map[string]bool{},
			ExtraTypes:      map[string]string{},
		}
		fmt.Fprintln(os.Stderr, "Generating", destPath, "with default config (no config file found)")
	} else {
		return wrap(err)
	}

	data, err := Gen(lib, version, config)
	if err != nil {
		return wrap(err)
	}

	// TODO: mkdir parents?
	return wrap(os.WriteFile(destPath, data, rw_r__r__))
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
		name := snake_case_to_PascalCase(val.Name())
		// GIR enum names can start with digits because GTK_LICENSE_0BSD is a thing
		rune0, _ := utf8.DecodeRuneInString(name)
		if unicode.IsDigit(rune0) {
			name = "_" + name
		}

		p("  %s:%d\n", name, val.Value())
	}
	p("})\n")
}
func (self *Generator) processConstantInfo(ci *gi.ConstantInfo) {
	p := printerTo(&self.goBindings)

	name := ci.Name()
	if self.namespace == "Gdk" && strings.HasPrefix(name, "KEY_") {
		// KEY_ constants maybe deserve special treatment?
		p("(set Key_%s %d)\n", name[4:], ci.Value())
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
	// TODO: should I use function builder et al.
	p := printerTo(&self.goBindings)

	var args []*gi.ArgInfo
	for i, n := 0, ci.NumArg(); i < n; i++ {
		arg := ci.Arg(i)
		args = append(args, arg)
	}

	userdata := -1
	for i, arg := range args {
		if arg.Closure() != -1 {
			userdata = i
			break
		}

		// treat any void* as userdata o_O
		t := arg.Type()
		if t.Tag() == gi.TYPE_TAG_VOID && t.IsPointer() {
			userdata = i
			break
		}
	}

	if userdata == -1 {
		p("; blacklisted (no userdata): ")
	}

	name := ci.Name()
	// feels terrible forcing Ref on all callbacks, maybe I should just allow coercing
	// null/Ref to function types
	p("(type %s (fn [", name)
	for i, ri, n := 0, 0, len(args); i < n; i++ {
		if i == userdata {
			continue
		}

		// idk what typeReturn is for honestly
		// from go-gir:
		// > I use here TypeReturn because for closures it's inverted
		// > C code calls closure and it has to be concrete and Go code
		// > returns stuff to C (like calling a C function)
		arg := args[i]
		if ri != 0 {
			p(" ")
		}
		p("%s %s", arg.Name(), horType(arg.Type(), typeConfig{
			namespace: self.namespace,
			flags:     typeReturn,
		}))
		ri++
	}
	ret := ci.ReturnType()
	if ret.Tag() == gi.TYPE_TAG_VOID && !ret.IsPointer() {
		p("] {}))\n")
	} else {
		p("] %s))\n", horType(ci.ReturnType(), typeConfig{namespace: self.namespace}))
	}

	if userdata == -1 {
		return
	}
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

	needSelfArg := flags&gi.FUNCTION_IS_METHOD != 0
	if needSelfArg && len(self.methodOwner) == 0 {
		panic(fmt.Sprintf("tried processing a method %s but no current class", name))
	}
	container := fi.Container()
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

	if needSelfArg {
		if self.inStruct {
			p("(Ref %s)", (self.methodOwner[len(self.methodOwner)-1]))
		} else {
			p("Self")
		}
	}
	for i, arg := range fb.args {
		if i != 0 || needSelfArg {
			p(" ")
		}
		flags := typeNone
		p("%s", horType(arg.typeInfo, typeConfig{flags, self.namespace}))
	}

	if len(fb.args) > 0 || needSelfArg || self.inStruct /* methods in structs aren't gi.FUNCTION_IS_METHOD */ {
		p(" ")
	}

	// wrapper function return type

	switch len(fb.rets) {
	case 0:
		p("{}")

	case 1:
		if flags&gi.FUNCTION_IS_CONSTRUCTOR != 0 {
			if self.inStruct { // I'm not 100% sure this is correct
				p("(Ref %s)", container.Name())
			} else {
				p("%s", container.Name())
			}
		} else {
			p("%s", horType(fb.rets[0].typeInfo, typeConfig{typeNone, self.namespace}))
		}

	default:
		p("{")
		for i, ret := range fb.rets {
			if ret.index == -2 {
				p("err: Error, ")
				continue
			}

			if ret.index >= 0 {
				p("%s: %s, ", sanitize(ret.argInfo.Name()), horType(ret.typeInfo, typeConfig{typeNone, self.namespace}))
			} else {
				p("_%d: %s, ", i, horType(ret.typeInfo, typeConfig{typeNone, self.namespace}))
			}
		}
		p("}")
	}

	// wrapper function args

	p(") [")
	if needSelfArg {
		if self.inStruct {
			p("self_")
		} else {
			p("self")
		}
	}
	for i, arg := range fb.args {
		if i != 0 || needSelfArg {
			p(" ")
		}
		p("%s", sanitize(snake_case_to_camelCase(arg.argInfo.Name())))
	}

	// wrapper function body

	p("] ")

	conversionArgs := map[int]string{}
	p("(let [\n")
	for i, arg := range fb.orig_args {
		argName := sanitize(snake_case_to_camelCase(arg.Name()))
		if arg.Type().Tag() == gi.TYPE_TAG_INTERFACE && arg.Type().Interface().Type() == gi.INFO_TYPE_CALLBACK {
			closure := arg.Closure()
			destroy := arg.Destroy()
			cclosureName := argName + "_closure"
			if _, ok := conversionArgs[i]; ok {
				// cleanup functions also (sometimes?) count as calllbacks,
				// we skip those or it will override the actual cleanup function
				continue
			}

			p("    %s (toCClosure %s)\n", cclosureName, argName)
			conversionArgs[i] = cclosureName + ".func"
			if closure != -1 {
				conversionArgs[closure] = cclosureName + ".data"
			}
			if destroy != -1 {
				conversionArgs[destroy] = cclosureName + ".cleanup"
			}
		} else if arg.Direction() == gi.DIRECTION_OUT || arg.Direction() == gi.DIRECTION_INOUT {
			p("    %s (stackAlloc)\n", sanitize(snake_case_to_camelCase(arg.Name())))
		} else if arg.OwnershipTransfer() == gi.TRANSFER_CONTAINER || arg.OwnershipTransfer() == gi.TRANSFER_EVERYTHING {
			conversionArgs[i] = "(retain " + argName + ")"
		}
	}

	needsWrapUnowned := returnTypeNeedsWrapUnowned(fi)

	// call to extern function
	p("    ret ")
	if needsWrapUnowned {
		p("(own ")
	}
	p("(%s", fi.Symbol())

	if needSelfArg {
		if self.inStruct {
			p(" self_")
		} else {
			p(" self")
		}
	}
	for i, arg := range fb.orig_args {
		// TODO: does this work for all types skipped?
		if conversion, ok := conversionArgs[i]; ok {
			p(" %s", conversion)
		} else if slices.Contains(fb.skiplist, i) {
			p(" null")
		} else {
			p(" %s", sanitize(snake_case_to_camelCase(arg.Name())))
		}
	}

	p(")")
	if needsWrapUnowned {
		p(")")
	}
	p("\n  ]\n    ")

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
				p("    %s: %s,\n", sanitize(ret.argInfo.Name()), retValue(ret))
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

	if needSelfArg {
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
			// TODO: handle ownership transfer?
			extern("(Ref %s) ", horType(arg.Type(), typeConfig{typeNone, self.namespace}))
		} else {
			flag := typeExact
			if arg.OwnershipTransfer() == gi.TRANSFER_CONTAINER || arg.OwnershipTransfer() == gi.TRANSFER_EVERYTHING {
				// TODO: handle container vs all?
				flag |= typeArgOwned
			}

			extern("%s ", horType(arg.Type(), typeConfig{flag, self.namespace}))
		}
	}
	if fi.ReturnType().Tag() == gi.TYPE_TAG_VOID && !fi.ReturnType().IsPointer() {
		// non-pointer void return
		extern("{}) [")
	} else {
		// why tf was I using container name
		if flags&gi.FUNCTION_IS_CONSTRUCTOR != 0 {
			if self.inStruct {
				extern("(Ref %s)) [", container.Name())
			} else {
				extern("%s) [", container.Name())
			}
		} else {
			// TODO: TRANSFER_CONTAINER?
			flag := typeReturn
			if needsWrapUnowned {
				flag |= typeReturnUnowned
			}
			typ := horType(fi.ReturnType(), typeConfig{flag, self.namespace})
			extern("%s) [", typ)
		}
	}

	// extern arguments

	if needSelfArg {
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

	if self.namespace == "GObject" && oi.Name() == "Object" {
		fmt.Fprintln(&self.goBindings, "(type Object Std.Object)")
		return
	}

	name := oi.Name()
	p := printerTo(&self.goBindings)

	p("(class extern %s (", snake_case_to_PascalCase(name))

	if self.namespace == "GObject" && oi.Name() == "Object" {
		p("{}")
	}

	if oi.Parent() != nil {
		name := oi.Parent().Name()
		ns := oi.Parent().Namespace()
		if ns != self.namespace {
			p("%s.", snake_case_to_PascalCase(ns))
		}
		p("%s ", snake_case_to_PascalCase(name))
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

func returnTypeNeedsWrapUnowned(fi *gi.FunctionInfo) bool {
	retTy := fi.ReturnType()

	// TODO: gi.TRANSFER_CONTAINER?
	if fi.CallerOwns() != gi.TRANSFER_NOTHING ||
		// ignore functions with no return value
		(retTy.Tag() == gi.TYPE_TAG_VOID && !retTy.IsPointer()) ||
		tagIsValueType(retTy.Tag()) {
		return false
	}

	// edge cases not covered in [tagIsValueType] c:
	if retTy.Tag() == gi.TYPE_TAG_INTERFACE {
		ii := retTy.Interface()
		return ii.Type() != gi.INFO_TYPE_ENUM &&
			ii.Type() != gi.INFO_TYPE_FLAGS &&
			!retTy.IsPointer()
	}

	return true
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

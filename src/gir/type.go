package gir

import (
	"bytes"
	"fmt"

	"github.com/linuxdeepin/go-gir/generator/gi"
)

type typeFlags int

const (
	typeNone    typeFlags = 0
	typePointer typeFlags = 1 << iota
	typeReturn
	typeListMember
	typeReceiver
	typeExact
)

type typeConfig struct {
	flags     typeFlags
	namespace string
}

func (self typeConfig) withFlag(flags typeFlags) typeConfig {
	self.flags = flags
	return self
}

func horType(ti *gi.TypeInfo, cfg typeConfig) string {
	var out bytes.Buffer

	switch tag := ti.Tag(); tag {
	case gi.TYPE_TAG_VOID:
		if ti.IsPointer() {
			out.WriteString("opaque")
			break
		}
		panic("Non-pointer void type is not supported")
	case gi.TYPE_TAG_UTF8, gi.TYPE_TAG_FILENAME:
		if cfg.flags&typeExact != 0 {
			out.WriteString("opaque")
		} else {
			out.WriteString("Str")
		}
	case gi.TYPE_TAG_ARRAY:
		size := ti.ArrayFixedSize()
		out.WriteString(horType(ti.ParamType(0), cfg))
		if size != -1 {
			fmt.Fprintf(&out, "[%d]", size)
		} else {
			out.WriteString("[]")
		}
	case gi.TYPE_TAG_GLIST:
		out.WriteString(horType(ti.ParamType(0), cfg))
		out.WriteString("[]")
	case gi.TYPE_TAG_GSLIST:
		out.WriteString(horType(ti.ParamType(0), cfg))
		out.WriteString("[]")
	case gi.TYPE_TAG_GHASH:
		// out.WriteString("map[")
		// out.WriteString(horType(ti.ParamType(0), cfg.flags))
		// out.WriteString("]")
		// out.WriteString(horType(ti.ParamType(1), cfg.flags))
	case gi.TYPE_TAG_ERROR:
		// not used?
		// out.WriteString("error")
	case gi.TYPE_TAG_INTERFACE:
		// TODO
		if ti.IsPointer() {
			cfg.flags |= typePointer
		}
		out.WriteString(horTypeForInterface(ti.Interface(), cfg))
	default:
		// TODO
		if ti.IsPointer() {
			cfg.flags |= typePointer
		}
		out.WriteString(horTypeForTag(tag, cfg))
	}
	return out.String()
}

func horTypeForTag(tag gi.TypeTag, cfg typeConfig) string {
	var out bytes.Buffer
	p := printerTo(&out)

	if cfg.flags&typePointer != 0 {
		p("*")
	}

	if cfg.flags&typeExact != 0 {
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

func horTypeForInterface(bi *gi.BaseInfo, cfg typeConfig) string {
	var out bytes.Buffer
	p := printerTo(&out)
	ns := snake_case_to_PascalCase(bi.Namespace())

	if cfg.flags&typeListMember != 0 {
		switch bi.Type() {
		case gi.INFO_TYPE_OBJECT, gi.INFO_TYPE_INTERFACE:
			return horTypeForInterface(bi, cfg.withFlag(typePointer|typeReturn))
		default:
			return horTypeForInterface(bi, cfg.withFlag(typeReturn))
		}
	}

	switch t := bi.Type(); t {
	case gi.INFO_TYPE_OBJECT, gi.INFO_TYPE_INTERFACE:
		if cfg.flags&typeExact != 0 {
			// exact type for object/interface is always an unsafe.Pointer
			p("opaque")
			break
		}

		if cfg.flags&(typeReturn|typeReceiver) != 0 && cfg.flags&typePointer != 0 {
			// receivers and return values are actual types,
			// and a pointer most likely
			p("*")
		}
		// TODO:  check namespace
		// prepend foreign types with appropriate namespace
		p("%s.", ns)

		p(bi.Name())

	case gi.INFO_TYPE_CALLBACK:
		if cfg.flags&typeExact != 0 {
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
	if cfg.flags&typePointer != 0 /* && !config.is_disguised(fullnm) */ {
		p("*")
	}
	// TODO: check namespace
	p("%s.", ns)
	p(bi.Name())
	return out.String()
}

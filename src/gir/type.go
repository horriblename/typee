package gir

import (
	"bytes"
	"fmt"
	"strings"
	"unsafe"

	"github.com/linuxdeepin/go-gir/generator/gi"
)

type typeFlags int

const (
	typeNone    typeFlags = 0
	typePointer typeFlags = 1 << iota
	typeReturn
	typeReturnUnowned
	typeArgOwned
	typeListMember
	typeReceiver

	// typeExact is used to for extern type signatures, i.e.
	// the C type (or a representation in hor that is ABI
	// compatible)
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

func (self typeFlags) has(flags typeFlags) bool {
	return self&flags != 0
}

func horType(ti *gi.TypeInfo, cfg typeConfig) string {
	tag := ti.Tag()
	if tagIsValueType(tag) {
		// TODO
		if ti.IsPointer() {
			cfg.flags |= typePointer
		}
		return horTypeForValueTypeTag(tag, cfg)
	}
	return horTypeForRefTypes(ti, cfg)
}

func horTypeForRefTypes(ti *gi.TypeInfo, cfg typeConfig) string {
	var out bytes.Buffer

	switch tag := ti.Tag(); tag {
	case gi.TYPE_TAG_VOID:
		if ti.IsPointer() {
			out.WriteString(cfg.flags.maybeWrapOwnership("Opaque"))
			break
		}
		panic("Non-pointer void type is not supported")
	case gi.TYPE_TAG_UTF8, gi.TYPE_TAG_FILENAME:
		out.WriteString(cfg.flags.maybeWrapOwnership("Opaque"))
	case gi.TYPE_TAG_ARRAY:
		out.WriteString(cfg.flags.maybeWrapOwnership("Opaque"))
		// size := ti.ArrayFixedSize()
		// out.WriteString("[")
		//
		// out.WriteString(horType(ti.ParamType(0), cfg))
		//
		// if size != -1 {
		// 	fmt.Fprintf(&out, " %d", size)
		// }
		// out.WriteString("]")
	case gi.TYPE_TAG_GLIST:
		out.WriteString(cfg.flags.maybeWrapOwnership("Opaque"))
	case gi.TYPE_TAG_GSLIST:
		out.WriteString(cfg.flags.maybeWrapOwnership("Opaque"))
	case gi.TYPE_TAG_GHASH:
		// out.WriteString("map[")
		// out.WriteString(horType(ti.ParamType(0), cfg.flags))
		// out.WriteString("]")
		// out.WriteString(horType(ti.ParamType(1), cfg.flags))
		// panic("GHash not supported yet")
		out.WriteString(cfg.flags.maybeWrapOwnership("TempGHash"))
	case gi.TYPE_TAG_ERROR:
		// TODO: should be a GLib.Error
		if cfg.namespace == "GLib" {
			out.WriteString(cfg.flags.maybeWrapOwnership("Error"))
		} else {
			out.WriteString(cfg.flags.maybeWrapOwnership("GLib.Error"))
		}
	case gi.TYPE_TAG_INTERFACE:
		// TODO
		if ti.IsPointer() {
			cfg.flags |= typePointer
		}
		out.WriteString(horTypeForInterface(ti.Interface(), cfg))
	default:
		panic(fmt.Sprintf("horTypeForRefTypes called with a value type? %v", tag))
	}
	return out.String()
}

// For basic types (or their pointer form)
func horTypeForValueTypeTag(tag gi.TypeTag, cfg typeConfig) string {
	var out bytes.Buffer
	p := printerTo(&out)

	if cfg.flags.has(typePointer) {
		if cfg.flags.has(typeReturn) && cfg.flags.has(typeReturnUnowned) {
			p("(Unowend ")
		} else if cfg.flags.has(typeArgOwned) {
			p("(Owned ")
		}
		p("(Ref ")
	}

	if cfg.flags&typeExact != 0 {
		switch tag {
		case gi.TYPE_TAG_BOOLEAN:
			// TODO: this should be gint, or maybe it
			// doesn't matter if I fix the codegen to output
			// the right size
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
			if cfg.namespace != "GObject" {
				p("GObject.Type")
			} else {
				p("Type")
			}
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
			if cfg.namespace != "GObject" {
				p("GObject.Type")
			} else {
				p("Type")
			}
		case gi.TYPE_TAG_UNICHAR:
			p("U32")
		default:
			panic("unreachable")
		}
	}

	if cfg.flags.has(typePointer) {
		p(")")
		if cfg.flags.has(typeArgOwned) ||
			cfg.flags.has(typeReturn) && cfg.flags.has(typeReturnUnowned) {
			p(")")
		}
	}

	return out.String()
}

func horTypeForInterface(bi *gi.BaseInfo, cfg typeConfig) string {
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
		var out bytes.Buffer
		p := printerTo(&out)
		ns := bi.Namespace()
		if cfg.flags&(typeReturn|typeReceiver) != 0 && cfg.flags&typePointer != 0 {
			// receivers and return values are actual types,
			// and a pointer most likely
			// p("*")
		}

		if cfg.namespace != bi.Namespace() {
			p("%s.", ns)
		}

		// NOTE: objects are pointers in hor ABI-wise so we
		// just use the original type name even with
		// [typeExact] for readability
		p(bi.Name())
		return cfg.flags.maybeWrapOwnership(out.String())

	case gi.INFO_TYPE_CALLBACK:
		if cfg.flags&typeExact != 0 {
			return "Opaque"
		}
	case gi.INFO_TYPE_ENUM, gi.INFO_TYPE_FLAGS:
		return horTypeForBasicInterface(bi, cfg)
	case gi.INFO_TYPE_STRUCT, gi.INFO_TYPE_UNION:
	default:
		panic("horTypeForInterface: unexpected InfoType " + t.String())
	}

	return cfg.flags.maybeWrapOwnership(horTypeForBasicInterface(bi, cfg))
}

func horTypeForBasicInterface(bi *gi.BaseInfo, cfg typeConfig) string {
	out := strings.Builder{}
	p := printerTo(&out)
	ns := bi.Namespace()

	if cfg.namespace != bi.Namespace() {
		p("%s.", ns)
	}
	p("%s", bi.Name())

	if cfg.flags&typePointer != 0 /* && !config.is_disguised(fullnm) */ {
		return fmt.Sprintf("(Ref %s)", out.String())
	}

	return out.String()
}

// FIXME: I have not looked at typeSize* functions at ALL
func typeSize(ti *gi.TypeInfo, flags typeFlags) int {
	ptrsize := int(unsafe.Sizeof(unsafe.Pointer(nil)))
	switch tag := ti.Tag(); tag {
	case gi.TYPE_TAG_VOID:
		if ti.IsPointer() {
			return ptrsize
		}
		panic("Non-pointer void type is not supported")
	case gi.TYPE_TAG_UTF8, gi.TYPE_TAG_FILENAME, gi.TYPE_TAG_GLIST,
		gi.TYPE_TAG_GSLIST, gi.TYPE_TAG_GHASH:
		return ptrsize
	case gi.TYPE_TAG_ARRAY:
		size := ti.ArrayFixedSize()
		if size != -1 {
			return size * typeSize(ti.ParamType(0), flags)
		}
		return ptrsize
	case gi.TYPE_TAG_INTERFACE:
		if ti.IsPointer() {
			flags |= typePointer
		}
		return typeSizeForInterface(ti.Interface(), flags)
	default:
		if ti.IsPointer() {
			flags |= typePointer
		}
		return typeSizeForTag(tag, flags)
	}
}

func typeSizeForTag(tag gi.TypeTag, flags typeFlags) int {
	ptrsize := int(unsafe.Sizeof(unsafe.Pointer(nil)))
	if flags&typePointer != 0 {
		return ptrsize
	}

	switch tag {
	case gi.TYPE_TAG_BOOLEAN:
		return 4
	case gi.TYPE_TAG_INT8:
		return 1
	case gi.TYPE_TAG_UINT8:
		return 1
	case gi.TYPE_TAG_INT16:
		return 2
	case gi.TYPE_TAG_UINT16:
		return 2
	case gi.TYPE_TAG_INT32:
		return 4
	case gi.TYPE_TAG_UINT32:
		return 4
	case gi.TYPE_TAG_INT64:
		return 8
	case gi.TYPE_TAG_UINT64:
		return 8
	case gi.TYPE_TAG_FLOAT:
		return 4
	case gi.TYPE_TAG_DOUBLE:
		return 8
	case gi.TYPE_TAG_GTYPE:
		return ptrsize
	case gi.TYPE_TAG_UNICHAR:
		return 4
	}
	panic("unreachable: " + tag.String())
}

func (self typeFlags) maybeWrapOwnership(ty string) string {
	if self&typeReturn != 0 {
		if self&typeReturnUnowned != 0 {
			return "(Unowned " + ty + ")"
		}
	} else if self&typeArgOwned != 0 {
		return "(Owned " + ty + ")"
	}
	return ty
}

// Note that the [gi.TypeInfo] that wraps this tag may still be a pointer
// ([gi.TypeInfo.IsPointer]).
// Non-value tags may be [gi.TYPE_TAG_VOID] or boxed types.
//
// Note that enum has [gi.TYPE_TAG_INTERFACE] tag and therefore is not a value
// type. WE LOVE GOBJECT
func tagIsValueType(tag gi.TypeTag) bool {
	switch tag {
	case gi.TYPE_TAG_VOID, gi.TYPE_TAG_UTF8, gi.TYPE_TAG_FILENAME,
		gi.TYPE_TAG_ARRAY, gi.TYPE_TAG_GLIST, gi.TYPE_TAG_GSLIST,
		gi.TYPE_TAG_GHASH, gi.TYPE_TAG_ERROR, gi.TYPE_TAG_INTERFACE:
		return false
	default:
		return true
	}
}

func typeSizeForInterface(bi *gi.BaseInfo, flags typeFlags) int {
	ptrsize := int(unsafe.Sizeof(unsafe.Pointer(nil)))
	if flags&typePointer != 0 {
		return ptrsize
	}

	switch t := bi.Type(); t {
	case gi.INFO_TYPE_OBJECT, gi.INFO_TYPE_INTERFACE:
		return ptrsize
	case gi.INFO_TYPE_STRUCT:
		si := gi.ToStructInfo(bi)
		return si.Size()
	case gi.INFO_TYPE_UNION:
		ui := gi.ToUnionInfo(bi)
		return ui.Size()
	case gi.INFO_TYPE_ENUM, gi.INFO_TYPE_FLAGS:
		ei := gi.ToEnumInfo(bi)
		return typeSizeForTag(ei.StorageType(), flags)
	case gi.INFO_TYPE_CALLBACK:
		return ptrsize
	}
	panic("unreachable: " + bi.Type().String())
}

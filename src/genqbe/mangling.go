package genqbe

import (
	"strings"

	"github.com/horriblename/typee/src/can"
)

type mangleOpts struct {
	module can.ModuleName
	class  string
	name   string
}

func mangleName(m mangleOpts) string {
	if m.name == "" {
		panic("mangleName: empty name")
	}

	var b strings.Builder
	if m.module != "" {
		b.WriteString(string(m.module))
		b.WriteString(".")
	}
	if m.class != "" {
		b.WriteString(strings.ReplaceAll(m.class, "_", "__"))
		b.WriteRune('_')
	}

	b.WriteString(strings.ReplaceAll(m.name, "_", "__"))
	return b.String()
}

func mangledClassTypeGetter(module can.ModuleName, className string) string {
	return mangleName(mangleOpts{
		module: module,
		class:  className,
		name:   "get_type",
	})
}

func mangledNew(module can.ModuleName, className string) string {
	return mangleName(mangleOpts{
		module: module,
		class:  className,
		name:   "new",
	})
}

type typeName struct {
	module can.ModuleName
	name   string
}

func mangledClassIfaceInit(class typeName, iface typeName) string {
	return mangleName(mangleOpts{
		module: class.module,
		class:  class.name,
		name:   "_",
	}) + mangleName(mangleOpts{
		module: iface.module,
		class:  iface.name,
		name:   "init",
	})
}

// debugging function to mangle arbitrary string with spaces into a valid QBE
// identifier
func debugMangle(s string) string {
	return strings.ReplaceAll(s, " ", "_")
}

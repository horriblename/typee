package genqbe

import "strings"

type mangleOpts struct {
	module string
	class  string
	name   string
}

func mangleName(m mangleOpts) string {
	if m.name == "" {
		panic("mangleName: empty name")
	}

	var b strings.Builder
	if m.module != "" {
		b.WriteString(m.module)
		b.WriteString(".")
	}
	if m.class != "" {
		b.WriteString(strings.ReplaceAll(m.class, "_", "__"))
		b.WriteRune('_')
	}

	b.WriteString(strings.ReplaceAll(m.name, "_", "__"))
	return b.String()
}

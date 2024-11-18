package genqbe

import "strings"

type mangleOpts struct {
	class string
	name  string
}

func mangleName(m mangleOpts) string {
	if m.name == "" {
		panic("mangleName: empty name")
	}

	var b strings.Builder
	if m.class != "" {
		b.WriteString(strings.ReplaceAll(m.class, "_", "__"))
		b.WriteRune('_')
	}

	b.WriteString(strings.ReplaceAll(m.name, "_", "__"))
	return b.String()
}

package genqbe

import (
	"strings"

	"github.com/horriblename/typee/src/simplesub"
)

type mangleOpts struct {
	module simplesub.CanonName
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

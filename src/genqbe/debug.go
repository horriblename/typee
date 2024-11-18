package genqbe

import (
	"fmt"
	"strings"
)

func (ctx *ctx) DumpUserTypes() string {
	var b strings.Builder
	for name, t := range ctx.userTypes {
		b.WriteString(name)
		b.WriteString(".Layouts:\n")
		for fname, field := range t.Layouts {
			b.WriteString(fmt.Sprintf("  %s: %#v\n", fname, field.Type))
		}
	}

	return b.String()
}

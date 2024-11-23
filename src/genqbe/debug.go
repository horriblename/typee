package genqbe

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/horriblename/typee/src/genqbe/qbeil"
)

func (ctx *ctx) DumpUserTypes() string {
	var b strings.Builder
	for _, t := range ctx.userTypes {
		switch ty := t.(type) {
		case qbeil.StructType:
			dumpStructType(ty, &b)
		case qbeil.UnionType:

		default:
			panic(fmt.Sprintf("unexpected qbeil.AggregateType: %#v", t))
		}
	}

	return b.String()
}

func dumpStructType(t qbeil.StructType, b *strings.Builder) {
	b.WriteString(t.Name)
	b.WriteString(".Layouts:\n")
	for fname, field := range t.Layouts {
		b.WriteString(fmt.Sprintf("  %s: %#v\n", fname, field.Type))
	}
}

func dumpUnionType(t qbeil.UnionType, b *strings.Builder) {
	b.WriteString(t.Name)
	b.WriteString("(size ")
	b.WriteString(strconv.Itoa(t.Size))
	b.WriteString(") Variants:\n")
	for _, v := range t.Variants {
		b.WriteString(fmt.Sprintf(" %#v\n", v))
	}
}

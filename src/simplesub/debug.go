package simplesub

import (
	"fmt"
	"os"
	"strings"
)

var indentLvl = 0
var EnableTrace = false

func trace(format string, args ...interface{}) {
	if !EnableTrace {
		return
	}
	fmt.Fprintf(os.Stderr, strings.Repeat("  ", indentLvl))
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintln(os.Stderr)
}

func DebugTypeTable(typeTree map[int]TypeScheme) string {
	var b strings.Builder
	for id, typ := range typeTree {
		b.WriteString(fmt.Sprintf("%d: %s\n", id, typ))
	}
	return b.String()
}

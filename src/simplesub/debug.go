package simplesub

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/horriblename/typee/src/parse"
)

var indentLvl = 0
var EnableTrace = false

const colorOrange = "\x1b[33m"
const colorReset = "\x1b[0m"

func trace(format string, args ...interface{}) {
	if !EnableTrace {
		return
	}
	fmt.Fprintf(os.Stderr, "%s", strings.Repeat("  ", indentLvl))
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintln(os.Stderr)
}

func DebugTypeTable(typeTree map[int]TypeScheme) string {
	var b strings.Builder

	type entry struct {
		id  int
		typ TypeScheme
	}

	typs := make([]entry, 0, len(typeTree))
	for id, typ := range maps.All(typeTree) {
		typs = append(typs, entry{id, typ})
	}
	slices.SortFunc(typs, func(a entry, b entry) int {
		if a.id == b.id {
			return 0
		}
		if a.id < b.id {
			return -1
		}
		return 1
	})

	for _, e := range typs {
		b.WriteString(fmt.Sprintf("%d: %s\n", e.id, e.typ))
	}
	return b.String()
}

func DebugTypedTree(ast []parse.Expr, typeTree map[int]TypeScheme) string {
	var b strings.Builder
	for _, expr := range ast {
		b.WriteString(fmt.Sprintf("%s\n", expr.String()))
	}

	replaces := make([]string, 0, len(typeTree)*2)
	for id, typ := range typeTree {
		from := fmt.Sprintf("#%d ", id)
		to := fmt.Sprintf(colorOrange+"%s"+colorReset, typ)
		replaces = append(replaces, from, to)
	}

	r := strings.NewReplacer(replaces...)
	return r.Replace(b.String())
}

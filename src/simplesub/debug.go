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

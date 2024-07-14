package assert

import (
	"fmt"
	"strings"
)

func Ok(e error, msg ...any) {
	if e != nil {
		panic(fmt.Errorf("assertion failed (got error): %s\n%s", e, joinHint(msg)))
	}
}

func Eq[T comparable](a T, b T, msg ...any) {
	if a != b {
		panic(fmt.Errorf("failed assertion a == b:\n  left: %v\n  right: %v\n%s", a, b,
			joinHint(msg)))
	}
}

func True(b bool, msg ...any) {
	if !b {
		panic(fmt.Errorf("failed assertion\n%s", joinHint(msg)))
	}
}

func joinHint(msg ...any) string {
	if len(msg) == 0 {
		return ""
	}

	b := strings.Builder{}
	b.WriteString("Hint: ")

	for m := range msg {
		b.WriteString(fmt.Sprintf("%v", m))
		b.WriteString(" ")
	}

	return b.String()
}

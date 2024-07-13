package assert

import (
	"fmt"
	"strings"
)

func Ok(e error, msg ...any) {
	if e != nil {
		panic(fmt.Errorf("assertion failed (got error): %s\n%s", e, join(msg)))
	}
}

func Eq[T comparable](a T, b T, msg ...any) {
	if a != b {
		panic(fmt.Errorf("failed assertion a == b:\n  left: %v\n  right: %v\nHint: %s", a, b,
			join(msg)))
	}
}

func True(b bool, msg ...any) {
	if !b {
		panic(fmt.Errorf("failed assertion\n%s", join(msg)))
	}
}

func join(msg ...any) string {
	b := strings.Builder{}

	for m := range msg {
		b.WriteString(fmt.Sprintf("%v", m))
		b.WriteString(" ")
	}

	return b.String()
}

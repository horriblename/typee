package assert

import "fmt"

func Ok(e error) {
	if e != nil {
		panic(fmt.Errorf("assertion failed (got error): %s", e))
	}
}

func Eq[T comparable](a T, b T) {
	if a != b {
		panic(fmt.Errorf("failed assertion a == b:\n  left: %v\n  right: %v", a, b))
	}
}

func True(b bool) {
	if !b {
		panic(fmt.Errorf("failed assertion (boolean check)"))
	}
}

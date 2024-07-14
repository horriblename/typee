package assert

import (
	"testing"
)

type TestAsserts struct{ t *testing.T }

func NewTestAsserts(t *testing.T) TestAsserts {
	return TestAsserts{t: t}
}

func (t *TestAsserts) Ok(e error, msg ...any) {
	t.t.Helper()
	if e != nil {
		t.t.Fatalf("assertion failed (got error): %s\n%s", e, joinHint(msg))
	}
}

func (t *TestAsserts) Eq(a any, b any, msg ...any) {
	t.t.Helper()
	if a != b {
		t.t.Fatalf("failed assertion a == b: \n  left: %v\n  right: %v\n%s", a, b,
			joinHint(msg))
	}
}

func (t *TestAsserts) True(b bool, msg ...any) {
	t.t.Helper()
	if !b {
		t.t.Fatalf("failed assertion\n%s", joinHint(msg))
	}
}

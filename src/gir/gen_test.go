package gir

import (
	"testing"

	"github.com/horriblename/typee/src/assert"
)

func TestThing(t *testing.T) {
	assert := assert.NewTestAsserts(t)

	bindings, err := New("GObject", "2.0", Config{})
	assert.Ok(err)

	t.Log(string(bindings))
}

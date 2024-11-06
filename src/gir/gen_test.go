package gir

import (
	"testing"

	"github.com/horriblename/typee/src/assert"
)

func TestThing(t *testing.T) {
	assert := assert.NewTestAsserts(t)

	g := Generator{namespace: "GObject"}
	_, err := g.Gen("GObject", "2.0")
	assert.Ok(err)

	t.Log(g.goBindings.String())
}

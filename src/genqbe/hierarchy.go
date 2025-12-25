package genqbe

import (
	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/types"
)

func (self *ctx) findMethodSource(c *types.Class, meth string) (_ *types.Class, ok bool) {
	if _, ok := c.Methods[meth]; ok {
		return c, true
	}

	for _, sup := range c.Supers {
		sty := self.resolveTypeApplications(sup)
		cty := assert.Cast[*types.Class](sty,
			"BUG non-object-type super caught during codegen ", sup)
		if cls, ok := self.findMethodSource(cty, meth); ok {
			return cls, true
		}
	}

	return nil, false
}

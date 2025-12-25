package simplesub

import (
	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/can"
)

// error can be nil if not found, in which case it means sub is not a subclass
// of target, but no other errors occured (i.e. undefined type/module).
func (self *symbols) isSubClass(sub ObjectType, target ObjectType) (bool, error) {
	assert.Neq(sub.Name, "", "BUG typer: unnamed class used as subclass in isSubClass")
	assert.Neq(sub.Name, "", "BUG typer: unnamed class used as superclass in isSubClass")

	for _, sup := range sub.Supers {
		// TODO: generics
		if sup.Module == target.Module && sup.Name == target.Name {
			return true, nil
		}

		ty, err := self.lookupType(sup.Module, sup.Name)
		if err != nil {
			return false, err
		}

		if found, err := self.isSubClass(ty.instantiate().(ObjectType), target); err != nil {
			return false, err
		} else if found {
			return true, nil
		}
	}

	return false, nil
}

// TODO: generics
func (self *symbols) getMethod(module can.ModuleName, class string, method string) (_ TypeScheme, found bool) {
	assert.Neq(class, "", "BUG getMethod called with empty class name")
	ts, ok := self.types.Get(class).Unwrap()
	if !ok {
		// FIXME: I'm pretty sure this can fail here
		panic("BUG unresolved type " + class)
	}
	if module != self.mainModule {
		mod := assert.Get(self.moduleCache, module, "BUG: module", module, "does not exist?")
		ts = assert.Get(mod.Types, class)
	}

	// FIXME: error intsead of assert?
	obj := assert.Cast[ObjectType](ts.instantiate())

	for _, meth := range obj.Methods {
		if method == meth.Name {
			return meth.Type, true
		}
	}

	for _, sup := range obj.Supers {
		meth, ok := self.getMethod(sup.Module, sup.Name, method)
		if ok {
			return meth, true
		}
	}

	return nil, false
}

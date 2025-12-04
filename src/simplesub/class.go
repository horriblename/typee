package simplesub

import "github.com/horriblename/typee/src/assert"

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

// Ordered collections
package ordered

import (
	"errors"
	"iter"
	"maps"
	"slices"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/fun"
)

var errListItemNotInMapping = errors.New(`invariant broken: an item in OrderedMap exists in list but not in mapping:`)

type Map[K comparable, V any] struct {
	list    []K
	mapping map[K]V
}

func NewMap[K comparable, V any]() Map[K, V] {
	return Map[K, V]{
		list:    []K{},
		mapping: map[K]V{},
	}
}

func CollectMap[K comparable, V any](seq iter.Seq2[K, V]) Map[K, V] {
	m := NewMap[K, V]()
	for k, v := range seq {
		m.Insert(k, v)
	}
	return m
}

func MapEq[K comparable, V comparable](self Map[K, V], other Map[K, V]) bool {
	if len(self.list) != len(other.list) {
		return false
	}
	for s, o := range fun.ZipSlicesStrict(self.list, other.list) {
		if s != o {
			return false
		}
	}

	for k1, v1 := range self.mapping {
		if v1 != other.mapping[k1] {
			return false
		}
	}

	return true
}

func MapEqFunc[K comparable, V any](self Map[K, V], other Map[K, V], eq func(V, V) bool) bool {
	if len(self.list) != len(other.list) {
		return false
	}
	for s, o := range fun.ZipSlicesStrict(self.list, other.list) {
		if s != o {
			return false
		}
	}

	for k1, v1 := range self.mapping {
		if !eq(v1, other.mapping[k1]) {
			return false
		}
	}

	return true
}
func (self *Map[K, V]) Insert(k K, v V) (existed bool) {
	_, existed = self.mapping[k]
	if !existed {
		self.mapping[k] = v
		self.list = append(self.list, k)
	}

	return existed
}

func (self Map[K, V]) With(k K, v V) Map[K, V] {
	if _, ok := self.mapping[k]; ok {
		return self
	}
	self.mapping[k] = v
	self.list = append(self.list, k)
	return self
}

func (self Map[K, V]) Get(x K) (value V, found bool) {
	value, found = self.mapping[x]
	return value, found
}

func (self Map[K, V]) Keys() []K {
	return self.list
}

func (self Map[K, V]) Len() int {
	return len(self.list)
}

func (self Map[K, V]) Pop() (t K, v V, has bool) {
	var zero K
	if len(self.list) > 0 {
		ret := self.list[len(self.list)-1]
		self.list[len(self.list)-1] = zero
		self.list = self.list[:len(self.list)-1]
		v = assert.Get(self.mapping, ret, errListItemNotInMapping, ret)
		delete(self.mapping, ret)
		return ret, v, true
	}

	return zero, v, false
}

func (self Map[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, k := range self.list {
			v := assert.Get(self.mapping, k, errListItemNotInMapping, k)
			if !yield(k, v) {
				return
			}
		}
	}
}

func (self Map[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, k := range self.list {
			v := assert.Get(self.mapping, k, errListItemNotInMapping, k)
			if !yield(v) {
				return
			}
		}
	}
}

func (self Map[K, V]) Map(f func(V) V) Map[K, V] {
	ret := NewMap[K, V]()
	for k, v := range self.All() {
		ret.Insert(k, f(v))
	}
	return ret
}

func (self Map[K, V]) Clone() Map[K, V] {
	return Map[K, V]{
		list:    slices.Clone(self.list),
		mapping: maps.Clone(self.mapping),
	}
}

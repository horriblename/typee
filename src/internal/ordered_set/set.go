package orderedset

type OrderedSet[T comparable] struct {
	list    []T
	mapping map[T]struct{}
}

func NewOrderedSet[T comparable](items ...T) *OrderedSet[T] {
	m := map[T]struct{}{}
	for _, v := range items {
		m[v] = struct{}{}
	}
	return &OrderedSet[T]{
		list:    items,
		mapping: m,
	}
}

func (set *OrderedSet[T]) Insert(x T) (existed bool) {
	_, existed = set.mapping[x]
	if !existed {
		set.mapping[x] = struct{}{}
		set.list = append(set.list, x)
	}

	return existed
}

func (set *OrderedSet[T]) Has(x T) bool {
	_, found := set.mapping[x]
	return found
}

func (set *OrderedSet[T]) Slice() []T {
	return set.list
}

func (set *OrderedSet[T]) Len() int {
	return len(set.list)
}

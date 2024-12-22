package orderedset

type OrderedSet[T comparable] struct {
	list    []T
	mapping map[T]int
}

func NewOrderedSet[T comparable](items ...T) *OrderedSet[T] {
	m := map[T]int{}
	for i, v := range items {
		m[v] = i
	}
	return &OrderedSet[T]{
		list:    items,
		mapping: m,
	}
}

func (set *OrderedSet[T]) Insert(x T) (existed bool) {
	_, existed = set.mapping[x]
	if !existed {
		set.mapping[x] = len(set.list)
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

func (set *OrderedSet[T]) Pop() (t T, has bool) {
	var zero T
	if len(set.list) > 0 {
		ret := set.list[len(set.list)-1]
		set.list[len(set.list)-1] = zero
		set.list = set.list[:len(set.list)-1]
		delete(set.mapping, ret)
		return ret, true
	}

	return zero, false
}

// swaps the position of t and the last element in the list, then delete t
func (set *OrderedSet[T]) SwapDelete(t T) bool {
	i, ok := set.mapping[t]
	if !ok {
		return false
	}

	if set.Len() == 1 {
		set.Pop()
		return true
	}

	oldLast := set.list[len(set.list)-1]
	set.mapping[oldLast] = i
	set.list[i], set.list[len(set.list)-1] = oldLast, set.list[i]
	set.Pop()

	return true
}

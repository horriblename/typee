package gir

func mapMap[K comparable, T, U any](xs map[K]T, f func(T) U) map[K]U {
	out := map[K]U{}
	for i, x := range xs {
		out[i] = f(x)
	}
	return out
}

func sliceToSet[T comparable](s []T) map[T]bool {
	set := map[T]bool{}
	for _, item := range s {
		set[item] = true
	}

	return set
}

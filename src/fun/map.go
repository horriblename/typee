package fun

func MapMap[K comparable, T, U any](m map[K]T, f func(T) U) map[K]U {
	out := map[K]U{}
	for k, v := range m {
		out[k] = f(v)
	}
	return out
}

func MapMapWithKeys[K comparable, T, U any](m map[K]T, f func(K, T) U) map[K]U {
	out := map[K]U{}
	for k, v := range m {
		out[k] = f(k, v)
	}
	return out
}

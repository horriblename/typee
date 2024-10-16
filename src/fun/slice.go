// functional utils
package fun

func Map[T, U any](xs []T, f func(T) U) []U {
	ys := make([]U, 0, len(xs))

	for _, x := range xs {
		ys = append(ys, f(x))
	}

	return ys
}

func ZipMap[T, U, V any](xs []T, ys []U, f func(T, U) V) []V {
	sz := min(len(xs), len(ys))
	zs := make([]V, 0, sz)

	for i, x := range xs[:sz] {
		y := ys[i]
		zs = append(zs, f(x, y))
	}

	return zs
}

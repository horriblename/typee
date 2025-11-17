// functional utils
package fun

import (
	"fmt"
	"iter"
	"slices"
)

func Map[T, U any](xs []T, f func(T) U) []U {
	ys := make([]U, 0, len(xs))

	for _, x := range xs {
		ys = append(ys, f(x))
	}

	return ys
}

func MapIfOk[T, U any](xs []T, f func(T) (U, error)) ([]U, error) {
	ys := make([]U, 0, len(xs))

	for _, x := range xs {
		y, err := f(x)
		if err != nil {
			return nil, err
		}

		ys = append(ys, y)
	}

	return ys, nil
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

type Pair[T, U any] struct {
	One T
	Two U
}

// panics if given iterators have different size
func ZipIterStrict[T, U any](i1 iter.Seq[T], i2 iter.Seq[U]) iter.Seq2[T, U] {
	return func(yield func(T, U) bool) {
		pull2, stop2 := iter.Pull(i2)
		defer stop2()

		for item1 := range i1 {
			item2, ok2 := pull2()
			if !ok2 {
				panic(fmt.Sprintf("ZipIter: leftover elements in first iterator: %v", item1))
			}

			if !yield(item1, item2) {
				return
			}
		}
		item2, ok2 := pull2()
		if ok2 {
			panic(fmt.Sprintf("ZipIter: leftover elements in second iterator: %v", item2))
		}
	}
}

// panics if given iterators have different size
func ZipSlicesStrict[T, U any](a []T, b []U) iter.Seq2[T, U] {
	return ZipIterStrict(slices.Values(a), slices.Values(b))
}

func Collect[T any](s iter.Seq[T]) []T {
	out := []T{}
	for x := range s {
		out = append(out, x)
	}
	return out
}

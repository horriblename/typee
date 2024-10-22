// package result provides a Result[T] type and helper functions. Use sparingly
package fun

type Result[T any] struct {
	err     error
	payload T
}

func Ok[T any](x T) Result[T] {
	return Result[T]{
		err:     nil,
		payload: x,
	}
}

func Err[T any](err error) Result[T] {
	if err == nil {
		panic("tried to construct a result using nil error")
	}
	return Result[T]{err: err}
}

func ResultFrom[T any](x T, e error) Result[T] {
	return Result[T]{e, x}
}

func (r Result[T]) Unwrap() (T, error) {
	return r.payload, r.err
}

func (r Result[T]) AssertWith(f func(error) interface{}) T {
	if r.err != nil {
		panic(f(r.err))
	}

	return r.payload
}

func BubbleResultSlice[T any](xs []Result[T]) ([]T, error) {
	ret := make([]T, 0, len(xs))
	for _, res := range xs {
		x, err := res.Unwrap()
		if err != nil {
			return nil, err
		}
		ret = append(ret, x)
	}

	return ret, nil
}

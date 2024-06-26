package opt

type Option[T any] struct {
	value T
	valid bool
}

func (o Option[T]) Unwrap() (T, bool) {
	return o.value, o.valid
}

func Some[T any](v T) Option[T] {
	return Option[T]{
		value: v,
		valid: true,
	}
}

func None[T any]() Option[T] {
	return Option[T]{}
}

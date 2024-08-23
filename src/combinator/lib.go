package combinator

import "github.com/horriblename/typee/src/opt"

type Parser[I any, O any] func(I) (I, O, error)

func Many0[I any, O any](parser Parser[I, O]) Parser[I, []O] {
	return func(in I) (I, []O, error) {
		outputs := make([]O, 0)
		for {
			rest, o, err := parser(in)
			if err != nil {
				break
			}

			outputs = append(outputs, o)
			in = rest
		}

		return in, outputs, nil
	}
}

func Many[I any, O any](parser Parser[I, O]) Parser[I, []O] {
	return func(in I) (I, []O, error) {
		outputs := make([]O, 0)
		in, o, err := parser(in)
		if err != nil {
			return in, outputs, err
		}
		outputs = append(outputs, o)

		for {
			rest, o, err := parser(in)
			if err != nil {
				break
			}

			outputs = append(outputs, o)
			in = rest
		}

		return in, outputs, nil
	}
}

func Any[I any, O any](parsers ...Parser[I, O]) Parser[I, O] {
	return func(i I) (I, O, error) {
		var o O
		var err error

		for _, parser := range parsers {
			var rest I
			rest, o, err = parser(i)
			if err == nil {
				return rest, o, err
			}
		}

		return i, o, ErrNoMatch
	}
}

func WithSuffix[I any, O any, O2 any](main Parser[I, O], suffix Parser[I, O2]) Parser[I, O] {
	return func(i I) (I, O, error) {
		rest, out, err := main(i)
		if err != nil {
			return rest, out, err
		}

		rest, _, err = suffix(rest)
		return rest, out, err
	}
}

func Surround[I, O, O1, O2 any](left Parser[I, O1], main Parser[I, O], right Parser[I, O2]) Parser[I, O] {
	return func(i I) (I, O, error) {
		var out O
		rest, _, err := left(i)
		if err != nil {
			return i, out, err
		}

		rest, out, err = main(rest)
		if err != nil {
			return i, out, err
		}

		rest, _, err = right(rest)
		if err != nil {
			return i, out, err
		}

		return rest, out, err
	}
}

type Pair[T, U any] struct {
	One T
	Two U
}

func Then[I, O1, O2 any](first Parser[I, O1], second Parser[I, O2]) Parser[I, Pair[O1, O2]] {
	return func(i I) (I, Pair[O1, O2], error) {
		i, o1, err := first(i)
		if err != nil {
			return i, Pair[O1, O2]{}, ErrNoMatch
		}

		i, o2, err := second(i)
		return i, Pair[O1, O2]{o1, o2}, err
	}
}

func SeperatedBy[I, O1, O2, O3 any](left Parser[I, O1], sep Parser[I, O2], right Parser[I, O3]) Parser[I, Pair[O1, O3]] {
	return func(i I) (I, Pair[O1, O3], error) {
		i, o1, err := left(i)
		if err != nil {
			return i, Pair[O1, O3]{}, ErrNoMatch
		}

		i, _, err = sep(i)
		if err != nil {
			return i, Pair[O1, O3]{}, ErrNoMatch
		}

		i, o3, err := right(i)
		return i, Pair[O1, O3]{o1, o3}, err
	}
}

// Parse list of item delimited by sep with optional sep at the end
func Delimited[I, O1, O2 any](item Parser[I, O1], sep Parser[I, O2]) Parser[I, []O1] {
	return func(i I) (i0 I, _ []O1, _ error) {
		results := []O1{}
		for {
			i1, o, err := item(i)
			if err != nil {
				return i, results, nil
			}

			i = i1
			results = append(results, o)

			i1, _, err = sep(i)
			if err != nil {
				return i, results, nil
			}

			i = i1
		}
	}
}

func Maybe[I, O any](parser Parser[I, O]) Parser[I, opt.Option[O]] {
	return func(i I) (I, opt.Option[O], error) {
		rest, o, err := parser(i)
		if err != nil {
			return i, opt.None[O](), nil
		}
		return rest, opt.Some(o), nil
	}
}

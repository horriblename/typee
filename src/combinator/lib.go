package combinator

type Parser[I any, O any] func(I) (I, O, error)

func Many[I any, O any](parser Parser[I, O]) Parser[I, []O] {

	return func(i I) (I, []O, error) {
		outputs := make([]O, 0)
		rest, o, err := parser(i)

		for err == nil {
			outputs = append(outputs, o)
			rest, o, err = parser(rest)
		}

		return rest, outputs, err
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

package combinator

import "errors"

var ErrNoMatch = errors.New("does not match")

func MatchOne[T comparable, O any](in []T, item T, output O) (rest []T, out O, err error) {
	if len(in) == 0 || in[0] != item {
		return rest, out, ErrNoMatch
	}

	return in[1:], output, nil
}

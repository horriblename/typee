package biunify

import (
	"errors"
	"fmt"
)

type TypePair struct {
	Value
	Use
}

var ErrTypeMismatch = errors.New("wrong type")
var ErrNoSuchField = errors.New("no such field")
var ErrNoSuchVariant = errors.New("no such variant")

func CheckHeads(lhs VTypeHead, rhs UTypeHead, out []TypePair) (_ []TypePair, err error) {
	switch lhs := lhs.(type) {
	case VBool:
		if _, ok := rhs.(UBool); ok {
			return []TypePair{}, nil
		}
		return nil, fmt.Errorf("%w: tried to use %v, expected %v", ErrTypeMismatch, lhs, rhs)
	case VFunc:
		rhsBound, ok := rhs.(UFunc)
		if !ok {
			return nil, fmt.Errorf("%w: tried to use %v, expected %v", ErrTypeMismatch, lhs, rhs)
		}

		out = append(out, TypePair{lhs.Ret, rhsBound.Ret})
		for i, rhsArg := range rhsBound.Arg {
			out = append(out, TypePair{rhsArg, lhs.Arg[i]})
		}

		return out, nil
	case VObj:
		rhsBound, ok := rhs.(UObj)
		if !ok {
			return nil, fmt.Errorf("%w: tried to use %v, expected %v", ErrTypeMismatch, lhs, rhs)
		}

		if field1, ok := lhs.Fields[rhsBound.Field.Name]; ok {
			out = append(out, TypePair{field1, rhsBound.Field.Use})
		} else {
			return nil, fmt.Errorf("%w: %s", ErrNoSuchField, rhsBound.Field.Name)
		}

	case VTagged:
		rhsBound, ok := rhs.(UTagged)
		if !ok {
			return nil, fmt.Errorf("%w: tried to use %v, expected %v", ErrTypeMismatch, lhs, rhs)
		}

		if body2, ok := rhsBound.Variants[lhs.Name]; ok {
			out = append(out, TypePair{lhs.Value, body2})
		} else {
			return nil, fmt.Errorf("no variant %s in %v", lhs.Name, rhsBound)
		}
	}
	panic(fmt.Sprintf("unexpected biunify.VTypeHead: %#v", lhs))
}

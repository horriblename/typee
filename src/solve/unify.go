package solve

func unify(cs []Constraint) ([]Constraint, error) {
	if len(cs) == 0 {
		return cs, nil
	}

	c := cs[0]
	if c.lhs.Simple() && c.lhs.Eq(c.rhs) {
		return cs[1:], nil
	}

	panic("TODO")
}

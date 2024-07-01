package solve

type TypeID int

type TypeTable struct {
	Names     map[string]TypeID
	idCounter TypeID
}

func NewTypeTable() TypeTable {
	return TypeTable{
		Names: map[string]TypeID{
			"Int":  0,
			"Str":  1,
			"Bool": 2,
		},
		idCounter: TypeID(2),
	}
}

func (tt TypeTable) Get(name string) TypeID {
	if id, found := tt.Names[name]; found {
		return id
	}

	tt.Names[name] = tt.idCounter
	tt.idCounter++
	return tt.Names[name]
}

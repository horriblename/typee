package qbeil

type Type interface{ typ() }

type BaseType int

const (
	Word   BaseType = iota // 32-bit int
	Long                   // 64-bit int
	Single                 // 32-bit float
	Double                 // 64-bit float
)

func (BaseType) typ() {}

func (t BaseType) String() string {
	switch t {
	case Word:
		return "w"
	case Long:
		return "l"
	case Single:
		return "s"
	case Double:
		return "d"
	}

	panic("unreachable")
}

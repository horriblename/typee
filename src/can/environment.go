package can

type env struct {
	Home  ModuleID
	Types map[string]namedType
}

type namedType struct {
	kind declaredType
	Type
}

type declaredType int

const (
	declaredUnion declaredType = iota
	declaredClass
	declaredEnum
	declaredAlias
)

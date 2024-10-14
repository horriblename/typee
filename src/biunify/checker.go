package biunify

type Value struct {
	ID ID
}

type Use struct {
	ID ID
	_  struct{} // prevents accidental conversion between Value & Use
}

type NamedValue struct {
	Name  string
	Value Value
}

type NamedUse struct {
	Name string
	Use  Use
}

type TypeError interface {
	error
}

type TypeChecker interface {
	// creates a type variable
	Var() (Value, Use)

	Int() Value
	IntUse() Use

	Bool() Value
	BoolUse() Use

	Func(args []Use, ret Value) Value
	FuncUse(args []Value, ret Use) Use

	Obj(fields []NamedValue) Value
	ObjUse(field NamedUse) Use

	Tagged(tag string, expr Value) Value
	TaggedUse(branches []NamedUse) Use

	// creates subtype constraints between the type nodes
	// returns a type error if types passed are not compatible, and returns nothing
	// on success
	Flow(lhs Value, rhs Use) error
}

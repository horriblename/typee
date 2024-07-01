package solve

import (
	"github.com/horriblename/typee/src/parse"
)

type TypeVar struct {
	id         TypeID
	identifier bool
}

type Subst struct {
	Target TypeVar
	Repl   TypeVar
}

type Constraint struct {
	lhs ExprID
	rhs TypeVar
}

type ExprID = int

func initConstraints(node parse.Expr) ([]Constraint, error) {
	constraints := []Constraint{}
	tt := NewTypeTable()
	err := genConstraints(tt, &constraints, node)
	if err != nil {
		return nil, err
	}

	return constraints, nil
}

func genConstraints(tt TypeTable, constraints *[]Constraint, node parse.Expr) error {
	switch node.(type) {
	case *parse.IntLiteral:
		*constraints = append(*constraints, Constraint{
			lhs: node.ID(),
			rhs: TypeVar{
				id:         tt.Get("Bool"),
				identifier: true,
			},
		})
	default:
		panic("unhandled node type in genConstraints")
	}

	return nil
}

var gId = 0

func newId() int {
	id := gId
	gId++
	return id
}

package types

//go-sumtype:decl Type

import (
	"fmt"
	"strings"

	"github.com/horriblename/typee/src/assert"
)

type Type interface {
	type_()
	String() string
	Simple() bool
	Eq(other Type) bool
}

type TypeID int

type String struct{}
type Int struct{}
type Bool struct{}
type Record struct {
	Fields map[string]Type
}
type Func struct {
	Args []Type
	Ret  Type
}
type Generic struct {
	ID      TypeID
	Name    string // can be empty
	Comment string // can be empty
}
type TypeScheme struct {
	Over []Generic
	Body Type
}

func (*String) type_()     {}
func (*Int) type_()        {}
func (*Bool) type_()       {}
func (*Record) type_()     {}
func (*Func) type_()       {}
func (*Generic) type_()    {}
func (*TypeScheme) type_() {}

func (*String) Simple() bool     { return true }
func (*Int) Simple() bool        { return true }
func (*Bool) Simple() bool       { return true }
func (*Record) Simple() bool     { return false }
func (*Func) Simple() bool       { return false }
func (*Generic) Simple() bool    { return false }
func (*TypeScheme) Simple() bool { return false }

func (*String) Eq(other Type) bool {
	_, ok := other.(*String)
	return ok
}
func (*Int) Eq(other Type) bool {
	_, ok := other.(*Int)
	return ok
}
func (*Bool) Eq(other Type) bool {
	_, ok := other.(*Bool)
	return ok
}
func (f *Record) Eq(other Type) bool {
	o, ok := other.(*Record)
	if !ok {
		return false
	}

	if len(f.Fields) != len(o.Fields) {
		return false
	}

	for name, val := range f.Fields {
		oval, has := o.Fields[name]
		if !has {
			return false
		}

		if !val.Eq(oval) {
			return false
		}
	}

	return true
}
func (f *Func) Eq(other Type) bool {
	o, ok := other.(*Func)
	if !ok {
		return false
	}

	assert.Eq(len(f.Args), 1, "only single argument functions supported, type:", f)
	return f.Args[0].Eq(o.Args[0]) && f.Ret.Eq(o.Ret)
}
func (g *Generic) Eq(other Type) bool {
	o, ok := other.(*Generic)
	return ok && o.ID == g.ID
}
func (ts *TypeScheme) Eq(other Type) bool {
	panic("todo")
}

func (*String) String() string { return "String" }
func (*Int) String() string    { return "Int" }
func (*Bool) String() string   { return "Bool" }
func (r *Record) String() string {
	if len(r.Fields) == 0 {
		return "{}"
	}
	b := strings.Builder{}
	b.WriteString("{")
	for name, val := range r.Fields {
		b.WriteString(name)
		b.WriteString(": ")
		b.WriteString(val.String())
		b.WriteString(", ")
	}
	b.WriteString("}")

	return b.String()
}
func (f *Func) String() string {
	assert.True(len(f.Args) > 0)
	b := strings.Builder{}
	b.WriteString("(")
	b.WriteString(f.Args[0].String())
	for _, arg := range f.Args[1:] {
		b.WriteString(" , ")
		b.WriteString(arg.String())
	}

	b.WriteString(" -> ")
	b.WriteString(f.Ret.String())
	b.WriteString(")")

	return b.String()
}
func (g *Generic) String() string {
	if g.Name == "" {
		return fmt.Sprintf("t%d", g.ID)
	}
	return fmt.Sprintf("%s#%d", g.Name, g.ID)
}
func (ts *TypeScheme) String() string {
	var b strings.Builder
	b.WriteString("∀")
	for _, g := range ts.Over {
		b.WriteString(g.String())
		b.WriteString(". ")
	}
	b.WriteString(ts.Body.String())
	return b.String()
}

var genericIDCounter TypeID = 0

func NewGeneric(name string, comment string) *Generic {
	genericIDCounter++
	return &Generic{
		ID:      genericIDCounter,
		Name:    name,
		Comment: comment,
	}
}

// like Eq, but the specific value of [Generic.ID] is not used for equality.
// e.g. 'a -> 'a -> Int and 'b -> 'b -> Int are structurally equal
func StructuralEq(a, b Type) bool {
	return structuralEq(structuralEqCtx{make(map[TypeID]TypeID), make(map[TypeID]TypeID)},
		a, b)
}

// like [StructuralEq], but check equality on each (a[i], b[i]) pair, using the
// same context, meaning ListStructuralEq(['a, 'a], ['b, 'c]) is not equal,
// because 'a == 'b is established in the first pair, therefore 'a == 'c != 'b
// is not logical
func ListStructuralEq(a, b []Type) bool {
	if len(a) != len(b) {
		return false
	}

	ctx := structuralEqCtx{
		make(map[TypeID]TypeID),
		make(map[TypeID]TypeID),
	}
	for i := range a {
		if !structuralEq(ctx, a[i], b[i]) {
			return false
		}
	}

	return true
}

type structuralEqCtx struct {
	aToB map[TypeID]TypeID
	bToA map[TypeID]TypeID
}

func structuralEq(ctx structuralEqCtx, a, b Type) bool {
	switch a := a.(type) {
	case *Int, *String, *Bool:
		return a.Eq(b)
	case *Func:
		b, ok := b.(*Func)
		if !ok || len(b.Args) != len(a.Args) {
			return false
		}

		for i := range a.Args {
			if !structuralEq(ctx, a.Args[i], b.Args[i]) {
				return false
			}
		}

		return structuralEq(ctx, a.Ret, b.Ret)

	case *Generic:
		b, ok := b.(*Generic)
		if !ok {
			return false
		}

		expectB, aMapped := ctx.aToB[a.ID]
		expectA, bMapped := ctx.bToA[b.ID]

		if aMapped && bMapped {
			return expectA == a.ID && expectB == b.ID
		} else if !aMapped && !bMapped {
			ctx.aToB[a.ID] = b.ID
			ctx.bToA[b.ID] = a.ID
			return true
		} else {
			return false
		}
	case *TypeScheme:
		b, ok := b.(*TypeScheme)
		if !ok {
			return false
		}

		// TODO: check for "bound" status e.g. 'a . 'a != 'b . 'c
		structuralEq(ctx, a.Body, b.Body)
	case *Record:
		b, ok := b.(*Record)
		if !ok {
			return false
		}

		if len(a.Fields) != len(b.Fields) {
			return false
		}

		for name, aval := range a.Fields {
			bval, ok := b.Fields[name]
			if !ok {
				return false
			}

			if !structuralEq(ctx, aval, bval) {
				return false
			}
		}

		return true
	}

	panic("unreachable")
}

func Clone(typ Type) Type {
	switch t := typ.(type) {
	case *String:
		t2 := *t
		return &t2
	case *Int:
		t2 := *t
		return &t2
	case *Bool:
		t2 := *t
		return &t2
	case *Func:
		assert.Eq(len(t.Args), 1, "only single arg functions supported")
		arg := Clone(t.Args[0])
		ret := Clone(t.Ret)

		return &Func{Args: []Type{arg}, Ret: ret}
	case *Generic:
		return &Generic{ID: t.ID, Name: t.Name, Comment: t.Comment}
	case *TypeScheme:
		generics := make([]Generic, len(t.Over))
		copy(generics, t.Over)
		return &TypeScheme{Over: generics, Body: Clone(t.Body)}
	case *Record:
		return &Record{
			Fields: mapMap(t.Fields, Clone),
		}
	}

	panic("unreachable")
}

func mapMap[K comparable, V1, V2 any](m map[K]V1, f func(V1) V2) map[K]V2 {
	newMap := map[K]V2{}
	for k, v := range m {
		newMap[k] = f(v)
	}

	return newMap
}

type PrettyCtx struct {
	mapping map[TypeID]string
	counter int
}

func (ctx *PrettyCtx) get(id TypeID) string {
	if prettyName, ok := ctx.mapping[id]; ok {
		return prettyName
	}

	ctx.counter = ctx.counter + 1
	ctx.mapping[id] = fmt.Sprintf("t%d", ctx.counter)
	return ctx.mapping[id]
}

func (ctx *PrettyCtx) String(typ Type) string {
	if ctx.mapping == nil {
		ctx.mapping = map[TypeID]string{}
	}
	switch t := typ.(type) {
	case *String, *Int, *Bool:
		return t.String()
	case *Func:
		assert.Eq(len(t.Args), 1, "only single arg functions supported")
		return fmt.Sprintf("(%s -> %s)", ctx.String(t.Args[0]), ctx.String(t.Ret))
	case *Generic:
		if prettyName, ok := ctx.mapping[t.ID]; ok {
			return prettyName
		}

		ctx.counter = ctx.counter + 1
		ctx.mapping[t.ID] = fmt.Sprintf("t%d", ctx.counter)
		return ctx.mapping[t.ID]

	case *TypeScheme:
		var b strings.Builder
		b.WriteString("∀")
		for _, g := range t.Over {
			b.WriteString(ctx.String(&g))
			b.WriteString(". ")
		}
		b.WriteString(ctx.String(t.Body))
		return b.String()
	case *Record:
		var b strings.Builder
		b.WriteString("{")
		for k, field := range t.Fields {
			b.WriteString(k)
			b.WriteString(": ")
			b.WriteString(ctx.String(field))
			b.WriteString(", ")
		}
		b.WriteString("}")
		return b.String()
	}

	panic("unreachable")
}

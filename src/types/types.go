package types

//go-sumtype:decl Type

import (
	"fmt"
	"iter"
	"maps"
	"strconv"
	"strings"

	"github.com/horriblename/typee/src/can"
	"github.com/horriblename/typee/src/fun"
	orderedset "github.com/horriblename/typee/src/internal/ordered_set"
	"github.com/horriblename/typee/src/opt"
	"github.com/horriblename/typee/src/parse"
)

type Type interface {
	type_()
	String() string
	Simple() bool
	Eq(other Type) bool
}

type Member struct {
	Access parse.AccessLvl
	Type   Type
}

type TypeID int

type String struct{}
type Int struct {
	Signed  bool
	BitSize int64
}
type Float struct{}
type Bool struct{}
type Ref struct {
	Content opt.Option[Type] // None means opaque pointer
}
type Record struct {
	Fields map[string]Type
}
type Union struct {
	Name     string
	Variants *orderedset.OrderedSet[Type]
}
type Enum struct {
	Name   string
	Values map[string]int64
}
type Class struct {
	Module  can.ModuleName
	Kind    parse.ObjectKind
	Name    string
	Supers  []*Application
	Fields  map[string]Member
	Statics map[string]Member
	Methods map[string]Member
	Top     bool
}
type Array struct {
	Type Type
	Size uint
}
type Slice struct {
	Type Type
}
type Func struct {
	Args   []Type
	Ret    Type
	Method bool
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
type Application struct {
	Module can.ModuleName
	Name   string
	Params []Type
}
type Top struct{}
type Join struct {
	Lhs Type
	Rhs Type
}
type Inter struct {
	Lhs Type
	Rhs Type
}

func (*String) type_()      {}
func (*Int) type_()         {}
func (*Float) type_()       {}
func (*Bool) type_()        {}
func (*Ref) type_()         {}
func (*Record) type_()      {}
func (*Union) type_()       {}
func (*Enum) type_()        {}
func (*Class) type_()       {}
func (*Array) type_()       {}
func (*Slice) type_()       {}
func (*Func) type_()        {}
func (*Generic) type_()     {}
func (*TypeScheme) type_()  {}
func (*Application) type_() {}
func (*Top) type_()         {}
func (*Join) type_()        {}
func (*Inter) type_()       {}

func (*String) Simple() bool      { return true }
func (*Int) Simple() bool         { return true }
func (*Float) Simple() bool       { return true }
func (*Bool) Simple() bool        { return true }
func (*Ref) Simple() bool         { return true }
func (*Record) Simple() bool      { return false }
func (*Union) Simple() bool       { return false }
func (*Enum) Simple() bool        { return false }
func (*Class) Simple() bool       { return false }
func (*Array) Simple() bool       { return false }
func (*Slice) Simple() bool       { return false }
func (*Func) Simple() bool        { return false }
func (*Generic) Simple() bool     { return false }
func (*TypeScheme) Simple() bool  { return false }
func (*Application) Simple() bool { return false }
func (*Top) Simple() bool         { return true }  // only used by biunification
func (*Join) Simple() bool        { return false } // only used by biunification
func (*Inter) Simple() bool       { return false } // only used by biunification

func (*String) Eq(other Type) bool {
	_, ok := other.(*String)
	return ok
}
func (self *Int) Eq(other Type) bool {
	o, ok := other.(*Int)
	return ok && self.Signed == o.Signed && self.BitSize == o.BitSize
}
func (*Float) Eq(other Type) bool {
	_, ok := other.(*Float)
	return ok
}
func (*Bool) Eq(other Type) bool {
	_, ok := other.(*Bool)
	return ok
}
func (self *Ref) Eq(other Type) bool {
	o, ok := other.(*Ref)
	return ok && self.Content == o.Content
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
func (f *Union) Eq(other Type) bool {
	o, ok := other.(*Union)
	if !ok {
		return false
	}

	if f.Name != "" && f.Name == o.Name {
		return true
	}

	if f.Variants.Len() != o.Variants.Len() {
		return false
	}

	for _, val := range f.Variants.Slice() {
		if !o.Variants.Has(val) {
			return false
		}
	}

	return true
}
func (f *Enum) Eq(other Type) bool {
	o, ok := other.(*Enum)
	if !ok {
		return false
	}

	return f.Name == o.Name
}
func (f *Class) Eq(other Type) bool {
	o, ok := other.(*Class)
	if !ok {
		return false
	}

	// FIXME: infinite recursion?
	if f.Name != o.Name {
		return false
	}

	if len(f.Fields) != len(o.Fields) {
		return false
	}

	for name, val := range joinSeq2(maps.All(f.Fields), maps.All(f.Methods)) {
		oval, has := o.Fields[name]
		if !has {
			return false
		}

		if val.Access != oval.Access {
			return false
		}

		if !val.Type.Eq(oval.Type) {
			return false
		}
	}

	return true
}
func (self *Array) Eq(other Type) bool {
	o, ok := other.(*Array)
	return ok && self.Type.Eq(o.Type) && self.Size == o.Size
}
func (self *Slice) Eq(other Type) bool {
	o, ok := other.(*Array)
	return ok && self.Type.Eq(o.Type)
}
func (f *Func) Eq(other Type) bool {
	o, ok := other.(*Func)
	if !ok {
		return false
	}

	if len(f.Args) != len(o.Args) {
		return false
	}

	for i, farg := range f.Args {
		if !farg.Eq(o.Args[i]) {
			return false
		}
	}

	return f.Ret.Eq(o.Ret)
}
func (g *Generic) Eq(other Type) bool {
	o, ok := other.(*Generic)
	return ok && o.ID == g.ID
}
func (ts *TypeScheme) Eq(other Type) bool {
	panic("todo")
}
func (self *Application) Eq(other Type) bool {
	o, ok := other.(*Application)
	if !(ok && self.Module == o.Module && self.Name == o.Name) {
		return false
	}

	if len(self.Params) != len(o.Params) {
		return false
	}

	for a, b := range fun.ZipSlices(self.Params, o.Params) {
		if !a.Eq(b) {
			return false
		}
	}

	return true
}
func (self *Top) Eq(other Type) bool {
	_, ok := other.(*Top)
	return ok
}

// note: Eq not used in biunification (I think)
func (self *Join) Eq(other Type) bool {
	o, ok := other.(*Join)
	if !ok {
		return false
	}
	return self.Lhs.Eq(o.Lhs) && self.Rhs.Eq(o.Rhs)
}
func (self *Inter) Eq(other Type) bool {
	o, ok := other.(*Join)
	if !ok {
		return false
	}
	return self.Lhs.Eq(o.Lhs) && self.Rhs.Eq(o.Rhs)
}

func (*String) String() string { return "String" }
func (self *Int) String() string {
	if self.Signed {
		return "I" + strconv.FormatInt(self.BitSize, 10)
	} else {
		return "U" + strconv.FormatInt(self.BitSize, 10)
	}
}
func (*Float) String() string    { return "Float" }
func (*Bool) String() string     { return "Bool" }
func (self *Ref) String() string { return fmt.Sprintf("Ref %s", self.Content.Or(nil)) }
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
func (r *Union) String() string {
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("union %s{", r.Name))
	for i, name := range r.Variants.Slice() {
		if i != 0 {
			b.WriteRune(' ')
		}
		b.WriteString(name.String())
	}
	b.WriteString("}")

	return b.String()
}
func (self Enum) String() string {
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("enum %s{", self.Name))
	for name, val := range self.Values {
		b.WriteString(name)
		b.WriteRune(':')
		b.WriteString(strconv.FormatInt(val, 10))
		b.WriteRune(' ')
	}
	b.WriteString("}")
	return b.String()
}
func (r *Class) String() string {
	if r.Name == "" {
		return "_UnknownObjectType"
	} else {
		return fmt.Sprintf("%s.%s{...}", r.Module, r.Name)
	}
}
func (self *Array) String() string {
	return fmt.Sprintf("[%s %d]", self.Type.String(), self.Size)
}
func (self *Slice) String() string {
	return fmt.Sprintf("[%s]", self.Type.String())
}
func (f *Func) String() string {
	b := strings.Builder{}
	b.WriteString("(")

	if len(f.Args) > 0 {
		b.WriteString(f.Args[0].String())
		for _, arg := range f.Args[1:] {
			b.WriteString(" , ")
			b.WriteString(arg.String())
		}
	} else {
		b.WriteString("_empty")
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
func (self *Application) String() string {
	var b strings.Builder
	b.WriteString("(")
	if self.Module != "" {
		b.WriteString(string(self.Module))
		b.WriteString(".")
	}
	b.WriteString(self.Name)

	for _, param := range self.Params {
		b.WriteString(" ")
		b.WriteString(param.String())
	}

	b.WriteString(")")
	return b.String()
}
func (self *Top) String() string   { return "⊤" }
func (self *Join) String() string  { return fmt.Sprintf("(%s ∪ %s)", self.Lhs, self.Rhs) }
func (self *Inter) String() string { return fmt.Sprintf("(%s ∩ %s)", self.Lhs, self.Rhs) }

type deepPrintCtx struct {
	visited map[string]Type
	buf     strings.Builder
}

func DeepPrint(ty Type) string {
	ctx := deepPrintCtx{visited: map[string]Type{}}
	ctx.print(ty)
	return ctx.buf.String()
}

func (self *deepPrintCtx) print(ty Type) {
	switch t := ty.(type) {
	case *Class:
		self.printClass(t)
	case *Func:
		for i, arg := range t.Args {
			if i != 0 {
				self.buf.WriteString(", ")
			}
			self.print(arg)
		}
		self.buf.WriteString(" -> ")
		self.print(t.Ret)
	default:
		self.buf.WriteString(ty.String())
	}
}

func (self *deepPrintCtx) printClass(r *Class) {
	b := &self.buf

	if self.visited[r.Name] != nil {
		fmt.Fprintf(b, "%s{...}", r.Name)
		return
	}

	if r.Name == "" {
		b.WriteString("_UnknownObjectType")
	} else {
		b.WriteString(r.Name)
		b.WriteRune('(')
		b.WriteString(strings.Join(fun.Map(r.Supers, func(c *Application) string {
			return c.Name
		}), ", "))
		b.WriteString(")")
	}
	b.WriteString("{")
	b.WriteString("fields: ")
	for name, val := range r.Fields {
		fmt.Fprintf(b, "%s %s: %s,",
			val.Access.String(),
			name,
			val.Type.String(),
		)
	}
	b.WriteString("methods: ")
	for name, val := range r.Methods {
		fmt.Fprintf(b, "%s %s: %s,",
			val.Access.String(),
			name,
			val.Type.String(),
		)
	}
	b.WriteString("}")
}

func (r *Class) DeepPrint() string {
	b := strings.Builder{}
	if r.Name == "" {
		b.WriteString("_UnknownObjectType")
	} else {
		b.WriteString(r.Name)
		b.WriteRune('(')
		b.WriteString(strings.Join(fun.Map(r.Supers, func(c *Application) string {
			return c.Name
		}), ", "))
		b.WriteString(")")
	}
	b.WriteString("{")
	b.WriteString("fields: ")
	for name, val := range r.Fields {
		b.WriteString(val.Access.String())
		b.WriteRune(' ')
		b.WriteString(name)
		b.WriteString(": ")
		b.WriteString(val.Type.String())
		b.WriteString(", ")
	}
	b.WriteString("methods: ")
	for name, val := range r.Methods {
		b.WriteString(val.Access.String())
		b.WriteRune(' ')
		b.WriteString(name)
		b.WriteString(": ")
		b.WriteString(val.Type.String())
		b.WriteString(", ")
	}
	b.WriteString("}")

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
	return structuralEq(structuralEqCtx{
		make(map[TypeID]TypeID),
		make(map[TypeID]TypeID),
		map[string]struct{}{},
	},
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
		map[string]struct{}{},
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

	visitedClasses map[string]struct{}
}

func structuralEq(ctx structuralEqCtx, a, b Type) bool {
	switch a := a.(type) {
	case *Int, *Float, *String, *Bool, *Top:
		return a.Eq(b)
	case *Ref:
		b, ok := b.(*Ref)
		if !ok {
			return false
		}

		at, ok1 := a.Content.Unwrap()
		bt, ok2 := b.Content.Unwrap()
		if ok1 != ok2 {
			return false
		}

		return !ok1 || (ok1 && structuralEq(ctx, at, bt))
	case *Inter:
		b, ok := b.(*Inter)
		if !ok {
			return false
		}
		return structuralEq(ctx, a.Lhs, b.Lhs) && structuralEq(ctx, a.Rhs, b.Rhs)
	case *Join:
		b, ok := b.(*Join)
		if !ok {
			return false
		}
		return structuralEq(ctx, a.Lhs, b.Lhs) && structuralEq(ctx, a.Rhs, b.Rhs)
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
	case *Union:
		b, ok := b.(*Union)
		if !ok {
			return false
		}
		if a.Name != "" && a.Name == b.Name {
			return true
		}

		// FIXME: structural typing + untagged unions is a nightmare
		return false

	case *Enum:
		b, ok := b.(*Enum)
		if !ok {
			return false
		}
		return a.Name == b.Name

	case *Class:
		b, ok := b.(*Class)
		if !ok {
			return false
		}

		// FIXME: infinite recursion?
		if a.Name != b.Name {
			return false
		}

		if _, ok := ctx.visitedClasses[a.Name]; ok {
			// FIXME: doesn't handle generalized classes ughhhhhhhhhhhhh
			return true
		}
		ctx.visitedClasses[a.Name] = struct{}{}

		if len(a.Fields) != len(b.Fields) {
			return false
		}

		for name, aval := range a.Fields {
			bval, ok := b.Fields[name]
			if !ok {
				return false
			}
			if aval.Access != bval.Access {
				return false
			}
			if !structuralEq(ctx, aval.Type, bval.Type) {
				return false
			}
		}

		if len(a.Methods) != len(b.Methods) {
			return false
		}

		for name, aval := range a.Methods {
			bval, ok := b.Methods[name]
			if !ok {
				return false
			}
			if aval.Access != bval.Access {
				return false
			}
			if !structuralEq(ctx, aval.Type, bval.Type) {
				return false
			}
		}
		return true
	case *Array:
		b, ok := b.(*Array)
		return ok && a.Size == b.Size && structuralEq(ctx, a.Type, b.Type)
	case *Slice:
		b, ok := b.(*Slice)
		return ok && structuralEq(ctx, a.Type, b.Type)
	case *Application:
		b, ok := b.(*Application)
		if !ok {
			return false
		}

		if a.Module != b.Module || a.Name != b.Name {
			return false
		}

		if len(a.Params) != len(b.Params) {
			return false
		}

		for pa, pb := range fun.ZipSlices(a.Params, b.Params) {
			if !structuralEq(ctx, pa, pb) {
				return false
			}
		}
		return true
	}

	panic(fmt.Sprintf("unexpected types.Type: %T", a))
}

// TODO: remove
func Clone(typ Type) Type {
	switch t := typ.(type) {
	case *String:
		t2 := *t
		return &t2
	case *Int:
		t2 := *t
		return &t2
	case *Float:
		t2 := *t
		return &t2
	case *Bool:
		t2 := *t
		return &t2
	case *Ref:
		t2 := *t
		return &t2
	case *Func:
		args := fun.Map(t.Args, Clone)
		ret := Clone(t.Ret)

		return &Func{Args: args, Ret: ret}
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

func (ctx *PrettyCtx) String(typ Type) string {
	if ctx.mapping == nil {
		ctx.mapping = map[TypeID]string{}
	}
	switch t := typ.(type) {
	case *String, *Int, *Float, *Bool, *Ref:
		return t.String()
	case *Func:
		args := fun.Map(t.Args, ctx.String)
		return fmt.Sprintf("(%s -> %s)", strings.Join(args, ", "), ctx.String(t.Ret))
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
	case *Union:
		var b strings.Builder
		b.WriteString("{")
		for i, t := range t.Variants.Slice() {
			if i != 0 {
				b.WriteString(" ")
			}
			b.WriteString(ctx.String(t))
		}
		b.WriteString("}")
		return b.String()
	case *Enum:
		var b strings.Builder
		b.WriteString("{")
		for name, val := range t.Values {
			b.WriteString(name)
			b.WriteString(":")
			b.WriteString(strconv.FormatInt(val, 10))
			b.WriteRune(' ')
		}
		b.WriteString("}")
		return b.String()
	}

	panic("unreachable")
}

func joinSeq2[K, V any](seqs ...iter.Seq2[K, V]) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, seq := range seqs {
			for k, v := range seq {
				if !yield(k, v) {
					return
				}
			}
		}
	}
}

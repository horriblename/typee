// The idea of canonicalization is to give all variables and types a
// unique "name" so that we can later cache information pertaining to
// the variable in type checking/optimization/other passes.
//
// NOTE: I am not doing real canonicalization (yet).
// currently this just stores information of user-defined types
// for later use
package can

import (
	"errors"
	"fmt"

	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/parse"
)

var ErrMissingMethodSignature = errors.New("class methods must have signature")

func Canonicalize(module string, ast []parse.Expr) (Module, error) {
	env := env{
		Home:  ModuleID(module),
		Types: map[string]TypeDef{},
	}
	return env.canonicalize(ast)
}

func (env *env) canonicalize(ast []parse.Expr) (Module, error) {
	if err := env.addTypes(ast); err != nil {
		return Module{}, err
	}

	return Module{
		Name: env.Home,
		Defs: env.Types,
	}, nil
}

func (env *env) addTypes(ast []parse.Expr) error {
	for _, expr := range ast {
		switch e := expr.(type) {
		case *parse.EnumDef:
			env.addEnum(e)

		case *parse.ObjectTypeDef:
			return env.addClass(e)

		case *parse.TypeAlias:
			env.addAlias(e)

		case *parse.UnionDef:
			env.addUnion(e)

		default:
		}
	}
	return nil
}

func (env *env) addEnum(def *parse.EnumDef) {
	env.Types[def.Name] = EnumType{
		Variants: fun.Map(def.Variants, func(v parse.EnumVariant) EnumVariant {
			return EnumVariant{
				Name:  TypeName(v.Name),
				Value: v.Value,
			}
		}),
	}

}

func (env *env) addClass(def *parse.ObjectTypeDef) error {
	fields := map[string]Member[Type]{}
	meths := map[string]Member[FnType]{}
	for _, field := range def.Fields {
		switch f := field.(type) {
		case parse.ClassField:
			fields[f.Name()] = Member[Type]{
				Type:   env.canonicalizeType(f.Type),
				Access: f.Access(),
			}

		case parse.ClassMethod:
			sig, ok := f.Func.Signature.Unwrap()
			if !ok {
				return fmt.Errorf("in %s.%s: %w", def.Name, f.Name(), ErrMissingMethodSignature)
			}
			meths[f.Name()] = Member[FnType]{
				Type: FnType{
					Args: fun.Map(sig[:len(sig)-1], func(arg parse.TypeRepr) Type {
						return env.canonicalizeType(arg)
					}),
					Ret: env.canonicalizeType(sig[len(sig)-1]),
				},
				Access: f.Access(),
			}

		default:
			panic(fmt.Sprintf("unexpected parse.ClassMember: %#v", field))
		}
	}

	env.Types[def.Name] = ClassType{
		Fields:  fields,
		Methods: meths,
	}

	return nil
}

func (env *env) addAlias(def *parse.TypeAlias) {
	env.Types[def.Name] = TypeAlias{
		Module: env.Home,
		Name:   def.Name,
		Type:   env.canonicalizeType(def.Type),
	}
}

func (env *env) addUnion(def *parse.UnionDef) {
	env.Types[def.Name] = UnionType{
		Variants: fun.Map(def.Variants, func(v parse.TypeRepr) Type {
			return env.canonicalizeType(v)
		}),
	}
}

func (env *env) canonicalizeType(tyRepr parse.TypeRepr) Type {
	switch ty := tyRepr.(type) {
	case parse.SelfType:
		return TypeApplication{Module: env.Home, Name: "Self"}

	case parse.RecordType:
		fields := map[string]Type{}
		for _, field := range ty.Fields {
			fields[field.Name] = env.canonicalizeType(field.Type)
		}
		return RecordType{
			Fields: fields,
		}

	case parse.ArrayType:
		if size, ok := ty.Size.Unwrap(); ok {
			return ArrayType{size, env.canonicalizeType(ty.Type)}
		}
		return SliceType{Type: env.canonicalizeType(ty.Type)}

	case parse.TypeInstantiation:
		return TypeApplication{
			Module: ModuleID(ty.Type.Module),
			Name:   ty.Type.Name,
			Params: fun.Map(ty.Params, func(param parse.TypeRepr) Type {
				return env.canonicalizeType(param)
			}),
		}

	case parse.TypeName:
		return TypeApplication{
			Module: ModuleID(ty.Module),
			Name:   ty.Name,
			Params: []Type{},
		}

	default:
		panic(fmt.Sprintf("unexpected parse.TypeRepr: %#v", ty))
	}
}

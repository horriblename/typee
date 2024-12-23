package parse

import (
	"errors"
	"fmt"

	"github.com/horriblename/typee/src/combinator"
	"github.com/horriblename/typee/src/lex"
)

var ErrParse = errors.New("parse error")
var LexError = errors.New("error in lex")
var ErrExpectEOF = errors.New("expected EOF")

type internalError struct{ error }

var gIdCounter = 1

func newId() int {
	id := gIdCounter
	gIdCounter++
	return id
}

func Program(in []lex.Token) ([]lex.Token, []Expr, error) {
	return combinator.Many(expr)(in)
}

func ParseString(source string) ([]Expr, error) {
	tokens, err := lex.LexString(source)
	if err != nil {
		return nil, errors.Join(LexError, err)
	}

	rest, prog, err := Program(tokens)
	if err != nil {
		return nil, err
	}

	if len(rest) != 0 {
		return nil, fmt.Errorf("%w: got token %s", ErrExpectEOF, rest[0:min(len(rest), 10)])
	}
	return prog, err
}

func expr(in []lex.Token) ([]lex.Token, Expr, error) {
	return combinator.Any(
		formLike,
		recordExpr,
		arrayLiteral,
		symbol,
		selfExpr,
		strLiteral,
		intLiteral,
		floatLiteral,
		kwTrue,
		kwFalse,
	)(in)
}

func strLiteral(in []lex.Token) ([]lex.Token, Expr, error) {
	if len(in) == 0 {
		return nil, nil, errAt(in)
	}

	if lit, ok := in[0].(*lex.StrLiteral); ok {
		return in[1:], &StrLiteral{id: newId(), Content: lit.Content}, nil
	}

	return nil, nil, errAt(in)
}

func intLiteral(in []lex.Token) ([]lex.Token, Expr, error) {
	if len(in) == 0 {
		return nil, nil, errAt(in)
	}

	if lit, ok := in[0].(*lex.IntLiteral); ok {
		return in[1:], &IntLiteral{id: newId(), Number: lit.Number}, nil
	}

	return nil, nil, errAt(in)
}

func floatLiteral(in []lex.Token) ([]lex.Token, Expr, error) {
	if len(in) == 0 {
		return nil, nil, errAt(in)
	}

	if lit, ok := in[0].(*lex.FloatLiteral); ok {
		return in[1:], &FloatLiteral{id: newId(), Number: lit.Number}, nil
	}

	return nil, nil, errAt(in)
}

func intNumber(in []lex.Token) ([]lex.Token, int64, error) {
	if len(in) == 0 {
		return nil, 0, errAt(in)
	}

	if lit, ok := in[0].(*lex.IntLiteral); ok {
		return in[1:], lit.Number, nil
	}

	return nil, 0, errAt(in)
}

func formLike(in []lex.Token) ([]lex.Token, Expr, error) {
	if len(in) == 0 {
		return nil, nil, errAt(in)
	}
	if _, ok := in[0].(*lex.LParen); !ok {
		return nil, nil, errAt(in)
	}

	switch at(in, 1).(type) {
	case *lex.Def:
		return defForm(in)

	case *lex.Set:
		return setForm(in)

	case *lex.Var:
		return varForm(in)

	case *lex.If:
		return ifExpr(in)

	case *lex.Let, *lex.LetRec:
		return letExpr(in)

	case *lex.Fn:
		return fnExpr(in)

	case *lex.Tag:
		return taggedExpr(in)

	case *lex.Case:
		return caseExpr(in)

	case *lex.Class:
		return classDef(in)

	case *lex.Interface:
		return interfaceDef(in)

	case *lex.Enum:
		return enumDef(in)

	case *lex.Union:
		return unionDef(in)

	case *lex.Type:
		return typeAlias(in)

	case *lex.CallExtern:
		return externCall(in)

	case *lex.Import:
		return importStmt(in)

	case nil:
		return nil, nil, errAt(in)

	default:
		return form(in)
	}
}

func form(in []lex.Token) (rest []lex.Token, exp Expr, err error) {
	rest, out, err := combinator.Surround(
		lparen,
		combinator.Many(expr),
		rparen,
	)(in)

	return rest, &Form{id: newId(), Children: out}, err
}

func externCall(in []lex.Token) (rest []lex.Token, exp Expr, err error) {
	rest, out, err := combinator.Surround(
		lparen,
		combinator.WithPrefix(
			kwCallExtern,
			combinator.Then(
				symbolName,
				combinator.Many0(expr),
			),
		),
		rparen,
	)(in)

	callee := Symbol{out.One, newId()}

	return rest, &ExternCall{
		id:     newId(),
		Symbol: callee,
		Args:   out.Two,
	}, err
}

func importStmt(in []lex.Token) ([]lex.Token, Expr, error) {
	in, path, err := combinator.Surround(
		lparen,
		combinator.WithPrefix(
			kwImport,
			combinator.Delimited(symbolName, dot),
		),
		rparen)(in)

	if err != nil {
		return nil, nil, err
	}

	return in, &Import{
		id:     newId(),
		Module: path,
	}, nil
}

func defForm(in []lex.Token) (_ []lex.Token, _ Expr, err error) {
	defer func() { err = handleCheck(recover(), err) }()

	in, _, err = lparen(in)
	check(err)

	in, _, err = kwDef(in)
	check(err)

	in, name, err := symbolName(in)
	check(err)

	// optional (T1 T2 ...)
	in, sig, err := combinator.Maybe(combinator.Surround(
		lparen,
		combinator.Many(type_),
		rparen,
	))(in)
	check(err)

	// [x y z ...]
	in, args, err := combinator.Surround(lbracket, combinator.Then(
		combinator.Maybe(kwSelf),
		combinator.Many0(symbolName),
	), rbracket)(in)
	check(err)

	in, body, err := combinator.Many(expr)(in)
	check(err)

	in, _, err = rparen(in)
	check(err)

	realArgs := args.Two
	if args.One.IsSome() {
		// so bad
		realArgs = append([]string{"self"}, realArgs...)
	}

	if sig, exist := sig.Unwrap(); exist && len(sig) != len(realArgs)+1 {
		return nil, nil, errors.New("function signature does not match arguments")
	}

	def := FuncDef{
		id:        newId(),
		Name:      name,
		Signature: sig,
		Args:      realArgs,
		Body:      body,
	}

	return in, &def, nil
}

func setForm(in []lex.Token) (_ []lex.Token, _ Expr, err error) {
	defer func() { err = handleCheck(recover(), err) }()

	in, _, err = lparen(in)
	check(err)

	in, _, err = kwSet(in)
	check(err)

	in, lval, err := symbolName(in)
	check(err)

	in, rval, err := expr(in)
	check(err)

	in, _, err = rparen(in)
	check(err)

	setExpr := &Set{
		id:    newId(),
		Name:  lval,
		Value: rval,
	}

	return in, setExpr, nil
}

func varForm(in []lex.Token) ([]lex.Token, Expr, error) {
	in, result, err := combinator.Surround(
		lparen,
		combinator.WithPrefix(
			kwVar,
			combinator.Then(
				symbolName,
				expr,
			),
		),
		rparen,
	)(in)

	if err != nil {
		return nil, nil, err
	}

	expr := &VarDef{
		id:    newId(),
		Name:  result.One,
		Value: result.Two,
	}

	return in, expr, nil
}

func ifExpr(in []lex.Token) (_ []lex.Token, _ Expr, err error) {
	defer func() { err = handleCheck(recover(), err) }()

	in, _, err = lparen(in)
	check(err)

	in, _, err = kwIf(in)
	check(err)

	in, cond, err := combinator.Surround(
		lbracket,
		expr,
		rbracket,
	)(in)
	check(err)

	in, cons, err := expr(in)
	check(err)

	in, alt, err := expr(in)
	check(err)

	in, _, err = rparen(in)
	check(err)

	return in, &IfExpr{
		id:          newId(),
		Condition:   cond,
		Consequence: cons,
		Alternative: alt,
	}, nil
}

func letExpr(in []lex.Token) (_ []lex.Token, _ Expr, err error) {
	defer func() { err = handleCheck(recover(), err) }()

	in, _, err = lparen(in)
	check(err)

	in, recursive, err := maybeRecursiveLet(in)
	check(err)

	in, ass, err := combinator.Surround(
		lbracket,
		combinator.Many0(assignment),
		rbracket,
	)(in)
	check(err)

	in, body, err := expr(in)
	check(err)

	in, _, err = rparen(in)
	check(err)

	let := &LetExpr{
		id:          newId(),
		Recursive:   recursive,
		Assignments: ass,
		Body:        body,
	}
	return in, let, nil
}

func assignment(in []lex.Token) (_ []lex.Token, _ Assignment, err error) {
	defer func() { err = handleCheck(recover(), err) }()

	in, name, err := symbolName(in)
	check(err)

	in, body, err := expr(in)
	check(err)

	return in, Assignment{Var: name, Value: body}, nil
}

func fnExpr(in []lex.Token) (_ []lex.Token, _ Expr, err error) {
	defer func() { err = handleCheck(recover(), err) }()

	in, _, err = lparen(in)
	check(err)

	in, _, err = kwFn(in)
	check(err)

	in, sig, err := combinator.Maybe(combinator.Surround(
		lparen,
		combinator.Many0(type_),
		rparen,
	))(in)

	in, args, err := combinator.Surround(
		lbracket,
		combinator.Many0(symbolName),
		rbracket,
	)(in)
	check(err)

	in, body, err := expr(in)
	check(err)

	in, _, err = rparen(in)
	check(err)

	return in, &Fn{Id: newId(), Signature: sig, Args: args, Body: body}, nil
}

func taggedExpr(in []lex.Token) (_ []lex.Token, _ Expr, err error) {
	defer func() { err = handleCheck(recover(), err) }()

	in, _, err = lparen(in)
	check(err)

	in, tag, err := tagName(in)

	in, body, err := expr(in)
	check(err)

	in, _, err = rparen(in)
	check(err)

	return in, &TaggedExpr{id: newId(), Tag: tag, Body: body}, nil
}

func caseExpr(in []lex.Token) (_ []lex.Token, _ Expr, err error) {
	defer func() { err = handleCheck(recover(), err) }()

	in, _, err = lparen(in)
	check(err)
	in, _, err = kwCase(in)
	check(err)

	in, match, err := expr(in)
	check(err)

	in, _, err = lbracket(in)
	check(err)

	in, cases, err := combinator.Many0(caseBranch)(in)

	in, _, err = rbracket(in)
	check(err)

	in, _, err = rparen(in)
	check(err)

	return in, &CaseExpr{
		id:       newId(),
		Match:    match,
		Branches: cases,
	}, nil
}

func casePattern(in []lex.Token) (_ []lex.Token, _ CasePattern, err error) {
	in, out, err := combinator.Surround(
		lparen,
		combinator.Then(
			tagName,
			symbolName,
		),
		rparen,
	)(in)

	if err != nil {
		return nil, CasePattern{}, err
	}

	return in, CasePattern{Tag: out.One, Pattern: out.Two}, nil
}

func caseBranch(in []lex.Token) (_ []lex.Token, _ CaseBranch, err error) {
	in, branch, err := combinator.Then(
		casePattern,
		expr,
	)(in)

	if err != nil {
		return nil, CaseBranch{}, err
	}

	return in, CaseBranch{
		Pattern: branch.One,
		Body:    branch.Two,
	}, nil
}

func recordExpr(in []lex.Token) (_ []lex.Token, _ Expr, err error) {
	defer func() { err = handleCheck(recover(), err) }()

	in, _, err = lbrace(in)
	check(err)

	in, pairs, err := combinator.Delimited(recordField, comma)(in)
	check(err)

	fields := make([]RecordField, 0, len(pairs))
	for _, pair := range pairs {
		fields = append(fields, RecordField{
			Name:  pair.One,
			Value: pair.Two,
		})
	}

	in, _, err = rbrace(in)
	check(err)

	return in, &Record{id: newId(), Fields: fields}, nil
}

func recordField(in []lex.Token) (_ []lex.Token, _ combinator.Pair[string, Expr], err error) {
	return combinator.SeperatedBy(symbolName, colon, expr)(in)
}

func arrayLiteral(in []lex.Token) ([]lex.Token, Expr, error) {
	in, out, err := combinator.Surround(
		lbracket,
		combinator.Many0(expr),
		rbracket,
	)(in)

	if err != nil {
		return nil, nil, err
	}

	return in, &ArrayLiteral{newId(), out}, nil
}

func selfLiteral(in []lex.Token) ([]lex.Token, Expr, error) {
	in, _, err := kwSelf(in)
	if err != nil {
		return nil, nil, err
	}

	return in, &SelfLiteral{newId()}, err
}

// SymbolExpr ::= Ident ('.' Ident)* ('.' 'new' | ”#' Ident | '::' Ident)
// dot accessor followed by other accessor is not currently implemented
func symbol(in []lex.Token) ([]lex.Token, Expr, error) {
	if len(in) == 0 {
		return nil, nil, errAt(in)
	}

	rest, lhs, err := combinator.Map(symbolName, func(n string) Expr {
		return &Symbol{n, newId()}
	})(in)
	if err != nil {
		return nil, nil, err
	}

	in = rest
	var acc Expr
	if rest, acc, err = recordAccess(lhs)(in); err == nil {
		lhs = acc
		in = rest
	}

	sym, ok := lhs.(*Symbol)
	if !ok {
		// TODO: allow dot accessor followed by other accessors
		return rest, lhs, nil
	}

	rest, accessor, err := combinator.Maybe(
		combinator.Any(
			methodAccess(lhs),
			classConstructor(sym.Name),
			enumAccess(sym.Name),
		),
	)(in)

	if err != nil {
		return nil, nil, err
	}

	if a, ok := accessor.Unwrap(); ok {
		return rest, a, nil
	}

	return in, lhs, nil
}

// returns error if there is no record access found
func recordAccess(lhs Expr) combinator.Parser[[]lex.Token, Expr] {
	return func(in []lex.Token) ([]lex.Token, Expr, error) {
		in, field, err := combinator.WithPrefix(dot, symbolName)(in)
		if err != nil {
			return nil, nil, err
		}

		newLhs := &RecordAccess{
			id:     newId(),
			Record: lhs,
			Field:  field,
		}

		rest, merged, err := recordAccess(newLhs)(in)
		if err != nil {
			// there is at least one record access, ignore error
			return in, newLhs, nil
		}
		return rest, merged, err
	}
}

func methodAccess(lhs Expr) combinator.Parser[[]lex.Token, Expr] {
	return combinator.Map(
		combinator.WithPrefix(hash, symbolName),
		func(member string) Expr {
			return &MethodAccess{
				id:     newId(),
				Var:    lhs,
				Method: member,
			}
		},
	)
}

func classConstructor(class string) combinator.Parser[[]lex.Token, Expr] {
	return combinator.Map(
		combinator.WithPrefix(dot, kwNew),
		func(struct{}) Expr {
			return &New{
				id:    newId(),
				Class: class,
			}
		},
	)
}

func enumAccess(enum string) combinator.Parser[[]lex.Token, Expr] {
	return combinator.Map(
		combinator.WithPrefix(doubleColon, symbolName),
		func(key string) Expr {
			return &EnumAccess{
				id:   newId(),
				Enum: enum,
				Key:  key,
			}
		},
	)
}

func selfExpr(in []lex.Token) ([]lex.Token, Expr, error) {
	in, s, err := selfLiteral(in)
	if err != nil {
		return nil, nil, err
	}

	rest, accessor, err := combinator.Any(
		recordAccess(s),
		methodAccess(s),
	)(in)

	if err != nil {
		return in, s, nil
	}

	return rest, accessor, nil
}

func symbolName(in []lex.Token) ([]lex.Token, string, error) {
	if len(in) == 0 {
		return nil, "", errAt(in)
	}

	if sym, ok := in[0].(*lex.Symbol); ok {
		return in[1:], sym.Name, nil
	} else {
		return nil, "", errAt(in)
	}
}

func tagName(in []lex.Token) ([]lex.Token, string, error) {
	if len(in) == 0 {
		return nil, "", errAt(in)
	}

	if sym, ok := in[0].(*lex.Tag); ok {
		return in[1:], sym.Label, nil
	}
	return nil, "", errAt(in)
}

func maybeRecursiveLet(in []lex.Token) (_ []lex.Token, recursive bool, err error) {
	if len(in) == 0 {
		return nil, false, errAt(in)
	}

	if _, ok := in[0].(*lex.Let); ok {
		return in[1:], false, nil
	}
	if _, ok := in[0].(*lex.LetRec); ok {
		return in[1:], true, nil
	}

	return nil, false, errAt(in)
}

func lparen(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.LParen])(in)
}
func rparen(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.RParen])(in)
}
func lbracket(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.LBracket])(in)
}
func rbracket(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.RBracket])(in)
}
func lbrace(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.LBrace])(in)
}
func rbrace(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.RBrace])(in)
}
func colon(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Colon])(in)
}
func doubleColon(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.DoubleColon])(in)
}
func comma(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Comma])(in)
}
func dot(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Dot])(in)
}
func hash(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Hash])(in)
}
func kwDef(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Def])(in)
}
func kwSet(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Set])(in)
}
func kwVar(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Var])(in)
}
func kwIf(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.If])(in)
}
func kwLet(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Let])(in)
}
func kwLetRec(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.LetRec])(in)
}
func kwImport(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Import])(in)
}
func kwClass(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Class])(in)
}
func kwInterface(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Interface])(in)
}
func kwFn(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Fn])(in)
}
func kwNew(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.New])(in)
}
func kwCase(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Case])(in)
}
func kwUnion(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Union])(in)
}
func kwEnum(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Enum])(in)
}
func kwSelf(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Self])(in)
}
func kwType(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.Type])(in)
}
func kwSelfType(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.SelfType])(in)
}
func kwCallExtern(in []lex.Token) ([]lex.Token, struct{}, error) {
	return wrappedResult(matchOne[*lex.CallExtern])(in)
}
func kwTrue(in []lex.Token) ([]lex.Token, Expr, error) {
	rest, _, err := wrappedResult(matchOne[*lex.TrueLiteral])(in)
	if err != nil {
		return nil, nil, err
	}

	return rest, &BoolLiteral{id: newId(), Value: true}, nil
}
func kwFalse(in []lex.Token) ([]lex.Token, Expr, error) {
	rest, _, err := wrappedResult(matchOne[*lex.FalseLiteral])(in)
	if err != nil {
		return nil, nil, err
	}

	return rest, &BoolLiteral{id: newId(), Value: false}, nil

}

func matchOne[T lex.Token](in []lex.Token) ([]lex.Token, struct{}, error) {
	if len(in) == 0 {
		return nil, struct{}{}, errAt(in)
	}

	if _, ok := in[0].(T); ok {
		return in[1:], struct{}{}, nil
	} else {
		return nil, struct{}{}, errAt(in)
	}
}

// returns in[idx] or nil if out of range
func at(in []lex.Token, idx int) lex.Token {
	if idx < 0 || idx >= len(in) {
		return nil
	}

	return in[idx]
}

func check(err error) {
	if err != nil {
		panic(&internalError{err})
	}
}

func handleCheck(err any, orig error) error {
	if err != nil {
		if err, ok := err.(*internalError); ok {
			return err.error
		} else {
			panic(err)
		}
	}

	return orig
}

// Error Handling

func errAt(in []lex.Token) error {
	return wrapIfErr(in, ErrParse)
}

func wrapIfErr(in []lex.Token, err error) error {
	if err == nil {
		return nil
	}

	if len(in) == 0 {
		return fmt.Errorf("at the end: %w", err)
	} else if len(in) <= 10 {
		return fmt.Errorf("at %v: %w", in, err)
	}

	return fmt.Errorf("at %v...: %w", in[:10], err)
}

func wrappedResult[I ~[]lex.Token, O any](parser combinator.Parser[I, O]) combinator.Parser[I, O] {
	return func(in I) (I, O, error) {
		rest, out, err := parser(in)
		return rest, out, wrapIfErr(in, err)
	}
}

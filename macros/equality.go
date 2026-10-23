package std_macros

import (
	"fmt"

	pm "github.com/goxide-lang/syntax/proc_macro"
	"github.com/goxide-lang/syntax/syn"
	"github.com/goxide-lang/syntax/token"
)

// Generic declarations are replayed unchanged. The target uses only parameter
// names, retaining each source identifier's binding and raw spelling.
func itemTarget(ctx pm.Context, item syn.Item) token.TokenStream {
	// A declaration name can originate in a macro_rules ident fragment. Refer to
	// its binding as an identifier here, rather than moving that opaque fragment
	// into a type slot. Field types and all other captured fragments stay intact.
	name := item.Name()
	identifier, err := token.NewIdent(name.Text(), name.Raw(), name.Context())
	if err != nil {
		panic(err)
	}
	result := token.New(identifier.WithSpan(name.Span()))
	parameters := item.Generics().Tokens()
	if len(parameters) == 0 {
		return result
	}
	var names []token.Token
	first := true
	for _, node := range parameters[0].Children().Tokens() {
		if node.Kind() == token.PunctKind && node.Text() == "," {
			first = true
			continue
		}
		if !first || node.Text() == "const" {
			continue
		}
		if len(names) > 0 {
			names = append(names, fragment(ctx, ",").Tokens()...)
		}
		names = append(names, node)
		first = false
	}
	return result.Append(group(ctx, token.Bracket, token.New(names...)))
}

func implHeader(ctx pm.Context, item syn.Item, protocol token.TokenStream) token.TokenStream {
	return fragment(ctx, "impl").Append(item.Generics().Tokens()...).Append(protocol.Tokens()...).Append(fragment(ctx, "for").Tokens()...).Append(itemTarget(ctx, item).Tokens()...)
}

func appendWhere(ctx pm.Context, item syn.Item, output, predicates token.TokenStream) token.TokenStream {
	where := item.Where()
	if !where.IsEmpty() {
		output = output.Append(where.Tokens()...)
		if !predicates.IsEmpty() {
			output = output.Append(fragment(ctx, ",").Tokens()...)
		}
	} else if !predicates.IsEmpty() {
		output = output.Append(fragment(ctx, "where").Tokens()...)
	}
	return output.Append(predicates.Tokens()...)
}

func addPredicate(ctx pm.Context, predicates, typ, protocol token.TokenStream) token.TokenStream {
	if !predicates.IsEmpty() {
		predicates = predicates.Append(fragment(ctx, ",").Tokens()...)
	}
	return predicates.Append(typ.Tokens()...).Append(fragment(ctx, ":").Tokens()...).Append(protocol.Tokens()...)
}

func deriveUnderlyingEquality(ctx pm.Context, item syn.Item, kind string) pm.Output {
	root, err := ctx.RuntimeRoot("github.com/goxide-lang/std")
	if err != nil {
		return pm.Failure(err.Error(), ctx.CallSpan())
	}
	protocol := root.Append(fragment(ctx, "::"+kind).Tokens()...)
	predicates := addPredicate(ctx, token.New(), item.Type().ToTokens(), protocol)
	var members token.TokenStream
	if kind == "PartialEq" {
		underlying := token.New(group(ctx, token.Parenthesis, item.Type().ToTokens()))
		left := underlying.Append(group(ctx, token.Parenthesis, fragment(ctx, "self")))
		right := underlying.Append(group(ctx, token.Parenthesis, fragment(ctx, "other")))
		args := left.Append(fragment(ctx, ",").Tokens()...).Append(right.Tokens()...)
		call := protocol.Append(fragment(ctx, "::equals").Tokens()...).Append(group(ctx, token.Parenthesis, args))
		members = fragment(ctx, "func equals(self Self,other Self) bool").Append(group(ctx, token.Brace, fragment(ctx, "return").Append(call.Tokens()...)))
	}
	if kind == "Eq" {
		predicates = addPredicate(ctx, predicates, itemTarget(ctx, item), root.Append(fragment(ctx, "::DerivedPartialEq").Tokens()...))
	}
	output := appendWhere(ctx, item, implHeader(ctx, item, protocol), predicates)
	return finishDerive(ctx, item, root, output.Append(group(ctx, token.Brace, members)), predicates, kind)
}

func variantPattern(ctx pm.Context, item syn.Item, variant syn.Variant, prefix string) (token.TokenStream, []token.TokenStream) {
	path := itemTarget(ctx, item).Append(fragment(ctx, "::").Tokens()...).Append(variant.Name().ToTokens().Tokens()...)
	var bindings token.TokenStream
	var values []token.TokenStream
	for _, field := range variant.Fields() {
		names := field.Names()
		if variant.Kind() == syn.TupleVariant {
			names = []syn.Ident{{}}
		}
		for _, name := range names {
			if !bindings.IsEmpty() {
				bindings = bindings.Append(fragment(ctx, ",").Tokens()...)
			}
			if variant.Kind() == syn.StructVariant {
				bindings = bindings.Append(name.ToTokens().Tokens()...).Append(fragment(ctx, ":").Tokens()...)
			}
			value := fragment(ctx, fmt.Sprintf("%s%d", prefix, len(values)))
			bindings = bindings.Append(value.Tokens()...)
			values = append(values, value)
		}
	}
	switch variant.Kind() {
	case syn.TupleVariant:
		path = path.Append(group(ctx, token.Parenthesis, bindings))
	case syn.StructVariant:
		path = path.Append(group(ctx, token.Brace, bindings))
	}
	return path, values
}

func deriveEnumEquality(ctx pm.Context, item syn.Item, root, protocol token.TokenStream, kind string) pm.Output {
	if len(item.Variants()) == 0 {
		return pm.Failure("cannot derive equality for an empty enum", item.Span())
	}
	var predicates, arms, members token.TokenStream
	for _, variant := range item.Variants() {
		for _, field := range variant.Fields() {
			predicates = addPredicate(ctx, predicates, field.Type().ToTokens(), protocol)
		}
		if kind == "Eq" {
			continue
		}
		left, leftValues := variantPattern(ctx, item, variant, "enumLeft")
		right, rightValues := variantPattern(ctx, item, variant, "enumRight")
		comparison := fragment(ctx, "true")
		for i := range leftValues {
			args := leftValues[i].Append(fragment(ctx, ",").Tokens()...).Append(rightValues[i].Tokens()...)
			call := protocol.Append(fragment(ctx, "::equals").Tokens()...).Append(group(ctx, token.Parenthesis, args))
			if i == 0 {
				comparison = call
			} else {
				comparison = comparison.Append(fragment(ctx, "&&").Tokens()...).Append(call.Tokens()...)
			}
		}
		inner := right.Append(fragment(ctx, "=>").Tokens()...).Append(comparison.Tokens()...).Append(fragment(ctx, ",").Tokens()...)
		if len(item.Variants()) > 1 {
			inner = inner.Append(fragment(ctx, "_=>false,").Tokens()...)
		}
		expression := fragment(ctx, "match other").Append(group(ctx, token.Brace, inner))
		arms = arms.Append(left.Tokens()...).Append(fragment(ctx, "=>").Tokens()...).Append(expression.Tokens()...).Append(fragment(ctx, ",").Tokens()...)
	}
	if kind == "PartialEq" {
		// Evaluate both values before entering this method, then validate the
		// entire finite left payload before the right payload or user equality.
		body := fragment(ctx, "__goxide_enum_assert(self); __goxide_enum_assert(other); return match self").Append(group(ctx, token.Brace, arms))
		members = fragment(ctx, "func equals(self Self,other Self) bool").Append(group(ctx, token.Brace, body))
	}
	if kind == "Eq" {
		predicates = addPredicate(ctx, predicates, itemTarget(ctx, item), root.Append(fragment(ctx, "::DerivedPartialEq").Tokens()...))
	}
	output := appendWhere(ctx, item, implHeader(ctx, item, protocol), predicates)
	return finishDerive(ctx, item, root, output.Append(group(ctx, token.Brace, members)), predicates, kind)
}

func assertTokens(ctx pm.Context, input token.TokenStream) pm.Output {
	if input.IsEmpty() {
		return pm.Failure("enum_assert requires one value", ctx.CallSpan())
	}
	output := fragment(ctx, "__goxide_enum_assert").Append(group(ctx, token.Parenthesis, input))
	return pm.Success(output)
}

func finishDerive(ctx pm.Context, item syn.Item, root, output, predicates token.TokenStream, kind string) pm.Output {
	if kind == "PartialEq" {
		marker := root.Append(fragment(ctx, "::DerivedPartialEq").Tokens()...)
		declaration := appendWhere(ctx, item, implHeader(ctx, item, marker), predicates).Append(group(ctx, token.Brace, token.New()))
		output = output.Append(token.NewSemicolon(ctx.DefSite())).Append(declaration.Tokens()...)
	}
	return pm.Success(output)
}

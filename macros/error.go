package std_macros

import (
	"fmt"
	"strconv"
	"strings"

	pm "github.com/goxide-lang/syntax/proc_macro"
	"github.com/goxide-lang/syntax/syn"
	"github.com/goxide-lang/syntax/token"
)

func errorSplice(ctx pm.Context, stream token.TokenStream, bindings map[string]token.TokenStream) token.TokenStream {
	result := token.New()
	for _, node := range stream.Tokens() {
		if value, ok := bindings[node.Text()]; ok && node.Kind() == token.IdentKind {
			result = result.Append(value.Tokens()...)
			continue
		}
		if node.Kind() == token.GroupKind {
			var err error
			node, err = token.NewGroup(node.Delimiter(), errorSplice(ctx, node.Children(), bindings), ctx.DefSite())
			if err != nil {
				panic(err)
			}
		}
		result = result.Append(node)
	}
	return result
}

func errorAttribute(attribute syn.Attribute, name string) bool {
	path := attribute.Path().Tokens()
	return len(path) == 1 && path[0].Text() == name
}

func errorMessage(message string, fields map[string]string) (string, error) {
	var parts []string
	var literal strings.Builder
	flush := func() {
		if literal.Len() != 0 {
			parts = append(parts, strconv.Quote(literal.String()))
			literal.Reset()
		}
	}
	for i := 0; i < len(message); {
		if strings.HasPrefix(message[i:], "{{") || strings.HasPrefix(message[i:], "}}") {
			literal.WriteByte(message[i])
			i += 2
			continue
		}
		if message[i] == '}' {
			return "", fmt.Errorf("unmatched closing brace in error message")
		}
		if message[i] != '{' {
			literal.WriteByte(message[i])
			i++
			continue
		}
		flush()
		end := strings.IndexByte(message[i+1:], '}')
		if end < 0 {
			return "", fmt.Errorf("unclosed error placeholder")
		}
		end += i + 1
		name := message[i+1 : end]
		field, ok := fields[name]
		if !ok {
			return "", fmt.Errorf("unknown error placeholder %q", name)
		}
		// An exact ordinary Go type-set enforces the builtin-scalar boundary.
		// The macro neither examines resolved types nor recognizes aliases.
		parts = append(parts, "__runtime::error_scalar("+field+")")
		i = end + 1
	}
	flush()
	if len(parts) == 0 {
		return strconv.Quote(""), nil
	}
	return strings.Join(parts, "+"), nil
}

type errorAttributes struct {
	message                  string
	hasMessage, source, from bool
}

func readErrorAttributes(attrs []syn.Attribute) (errorAttributes, error) {
	var result errorAttributes
	seen := map[string]bool{}
	for _, attr := range attrs {
		name := ""
		for _, candidate := range []string{"error", "source", "from"} {
			if errorAttribute(attr, candidate) {
				name = candidate
			}
		}
		if name == "" {
			continue
		}
		if seen[name] {
			return result, fmt.Errorf("duplicate %s attribute", name)
		}
		seen[name] = true
		if name != "error" {
			if !attr.Arguments().IsEmpty() {
				return result, fmt.Errorf("%s attribute takes no arguments", name)
			}
			if name == "source" {
				result.source = true
			} else {
				result.from = true
			}
			continue
		}
		args := attr.Arguments().Tokens()
		if len(args) != 1 || args[0].Kind() != token.GroupKind || args[0].Delimiter() != token.Parenthesis {
			return result, fmt.Errorf("error requires one string literal")
		}
		text := args[0].Children().Tokens()
		if len(text) != 1 || text[0].Kind() != token.LiteralKind || text[0].LiteralType() != token.String {
			return result, fmt.Errorf("error requires one string literal")
		}
		value, err := strconv.Unquote(text[0].Text())
		if err != nil {
			return result, err
		}
		result.message, result.hasMessage = value, true
	}
	return result, nil
}

func deriveError(ctx pm.Context, input token.TokenStream) pm.Output {
	item, err := syn.Parse(input)
	if err != nil {
		return pm.Failure(err.Error(), ctx.CallSpan())
	}
	if item.Alias() {
		return pm.Failure("Error requires a defined struct or enum", item.Span())
	}
	root, err := ctx.RuntimeRoot("github.com/goxide-lang/std")
	if err != nil {
		return pm.Failure(err.Error(), ctx.CallSpan())
	}
	if item.Kind() == syn.EnumItem {
		return deriveEnumError(ctx, item, root)
	}
	fields, ok := item.Type().StructFields()
	if !ok {
		return pm.Failure("Error requires a defined struct or enum", item.Span())
	}
	attrs, err := readErrorAttributes(item.Attributes())
	if err != nil {
		return pm.Failure(err.Error(), item.Span())
	}
	if !attrs.hasMessage || attrs.source || attrs.from {
		return pm.Failure("Error struct requires a message; source/from belong to a field", item.Span())
	}
	bindings := map[string]token.TokenStream{"__runtime": root}
	names := map[string]string{}
	source, from := "", false
	for _, field := range fields {
		if len(field.Names()) == 0 {
			return pm.Failure("embedded fields have no Error derive protocol", field.Span())
		}
		attr, err := readErrorAttributes(field.Attributes())
		if err != nil {
			return pm.Failure(err.Error(), field.Span())
		}
		if attr.hasMessage {
			return pm.Failure("error message belongs to the struct, not a field", field.Span())
		}
		for _, name := range field.Names() {
			if name.Text() == "_" {
				return pm.Failure("blank fields have no Error derive protocol", name.Span())
			}
			key := fmt.Sprintf("__field%d", len(names))
			bindings[key] = name.ToTokens()
			names[name.Text()] = "self." + key
			if attr.source || attr.from {
				if source != "" {
					return pm.Failure("Error requires at most one source field", field.Span())
				}
				source, from = key, attr.from
			}
		}
	}
	if from && len(names) != 1 {
		return pm.Failure("from requires exactly one error field", item.Span())
	}
	message, err := errorMessage(attrs.message, names)
	if err != nil {
		return pm.Failure(err.Error(), item.Span())
	}
	members := "pub func Error(self Self)string{return " + message + "}"
	if source != "" {
		members += ";pub func Unwrap(self Self)error{return __runtime::error_source(&self." + source + ")}"
	}
	header := fragment(ctx, "impl").Append(item.Generics().Tokens()...).Append(itemTarget(ctx, item).Tokens()...).Append(item.Where().Tokens()...)
	output := header.Append(group(ctx, token.Brace, errorSplice(ctx, fragment(ctx, members), bindings)))
	if from {
		protocol := root.Append(fragment(ctx, "::From[error]").Tokens()...)
		constructor := fragment(ctx, "Self").Append(group(ctx, token.Brace, bindings[source].Append(fragment(ctx, ":value").Tokens()...)))
		body := fragment(ctx, "return").Append(constructor.Tokens()...)
		fromMembers := fragment(ctx, "func from(value error)Self").Append(group(ctx, token.Brace, body))
		output = output.Append(token.NewSemicolon(ctx.DefSite())).Append(implHeader(ctx, item, protocol).Append(item.Where().Tokens()...).Append(group(ctx, token.Brace, fromMembers)).Tokens()...)
	}
	return pm.Success(output)
}

func deriveEnumError(ctx pm.Context, item syn.Item, root token.TokenStream) pm.Output {
	attrs, err := readErrorAttributes(item.Attributes())
	if err != nil {
		return pm.Failure(err.Error(), item.Span())
	}
	if attrs.hasMessage || attrs.source || attrs.from {
		return pm.Failure("enum error/source/from attributes belong to variants", item.Span())
	}
	if len(item.Variants()) == 0 {
		return pm.Failure("Error requires a nonempty enum", item.Span())
	}
	var errorArms, sourceArms, fromMember token.TokenStream
	for _, variant := range item.Variants() {
		attr, err := readErrorAttributes(variant.Attributes())
		if err != nil {
			return pm.Failure(err.Error(), variant.Span())
		}
		if !attr.hasMessage {
			return pm.Failure("Error variant requires an error message", variant.Span())
		}
		pattern, values := variantPattern(ctx, item, variant, "errorField")
		if (attr.source || attr.from) && len(values) != 1 {
			return pm.Failure("source/from variant requires one error payload", variant.Span())
		}
		if attr.from && !fromMember.IsEmpty() {
			return pm.Failure("Error requires at most one from variant", variant.Span())
		}
		names := map[string]string{}
		bindings := map[string]token.TokenStream{"__runtime": root}
		index := 0
		for _, field := range variant.Fields() {
			fieldAttr, err := readErrorAttributes(field.Attributes())
			if err != nil {
				return pm.Failure(err.Error(), field.Span())
			}
			if fieldAttr.hasMessage || fieldAttr.source || fieldAttr.from {
				return pm.Failure("enum error/source/from attributes belong to variants", field.Span())
			}
			fieldNames := field.Names()
			if variant.Kind() == syn.TupleVariant {
				label := "Value"
				if len(values) > 1 {
					label = fmt.Sprintf("V%d", index)
				}
				key := fmt.Sprintf("__field%d", index)
				names[label] = key
				bindings[key] = values[index]
				index++
				continue
			}
			if len(fieldNames) == 0 {
				return pm.Failure("named enum fields required", field.Span())
			}
			for _, name := range fieldNames {
				if name.Text() == "_" {
					return pm.Failure("blank fields have no Error derive protocol", name.Span())
				}
				key := fmt.Sprintf("__field%d", index)
				names[name.Text()] = key
				bindings[key] = values[index]
				index++
			}
		}
		message, err := errorMessage(attr.message, names)
		if err != nil {
			return pm.Failure(err.Error(), variant.Span())
		}
		expression := errorSplice(ctx, fragment(ctx, message), bindings)
		errorArms = errorArms.Append(pattern.Tokens()...).Append(fragment(ctx, "=>").Tokens()...).Append(expression.Tokens()...).Append(fragment(ctx, ",").Tokens()...)
		source := fragment(ctx, "nil")
		if attr.source || attr.from {
			source = root.Append(fragment(ctx, "::error_source").Tokens()...).Append(group(ctx, token.Parenthesis, fragment(ctx, "&").Append(values[0].Tokens()...)))
		}
		sourceArms = sourceArms.Append(pattern.Tokens()...).Append(fragment(ctx, "=>").Tokens()...).Append(source.Tokens()...).Append(fragment(ctx, ",").Tokens()...)
		if attr.from {
			constructor := itemTarget(ctx, item).Append(fragment(ctx, "::").Tokens()...).Append(variant.Name().ToTokens().Tokens()...)
			if variant.Kind() == syn.TupleVariant {
				constructor = constructor.Append(group(ctx, token.Parenthesis, fragment(ctx, "value")))
			} else {
				constructor = constructor.Append(group(ctx, token.Brace, variant.Fields()[0].Names()[0].ToTokens().Append(fragment(ctx, ":value").Tokens()...)))
			}
			fromMember = fragment(ctx, "func from(value error)Self").Append(group(ctx, token.Brace, fragment(ctx, "return").Append(constructor.Tokens()...)))
		}
	}
	errorBody := fragment(ctx, "return match self").Append(group(ctx, token.Brace, errorArms))
	sourceBody := fragment(ctx, "return match self").Append(group(ctx, token.Brace, sourceArms))
	members := fragment(ctx, "pub func Error(self Self)string").Append(group(ctx, token.Brace, errorBody)).Append(fragment(ctx, ";pub func Unwrap(self Self)error").Tokens()...).Append(group(ctx, token.Brace, sourceBody))
	header := fragment(ctx, "impl").Append(item.Generics().Tokens()...).Append(itemTarget(ctx, item).Tokens()...).Append(item.Where().Tokens()...)
	output := header.Append(group(ctx, token.Brace, members))
	if !fromMember.IsEmpty() {
		protocol := root.Append(fragment(ctx, "::From[error]").Tokens()...)
		output = output.Append(token.NewSemicolon(ctx.DefSite())).Append(implHeader(ctx, item, protocol).Append(item.Where().Tokens()...).Append(group(ctx, token.Brace, fromMember)).Tokens()...)
	}
	return pm.Success(output)
}

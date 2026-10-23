package std_macros

import (
	"fmt"
	"strconv"
	"strings"

	pm "github.com/goxide-lang/syntax/proc_macro"
	"github.com/goxide-lang/syntax/token"
)

type part struct {
	literal     string
	slot, debug bool
}

func parts(source string) ([]part, error) {
	var result []part
	var literal strings.Builder
	flush := func() {
		if literal.Len() > 0 {
			result = append(result, part{literal: literal.String()})
			literal.Reset()
		}
	}
	for i := 0; i < len(source); {
		switch {
		case strings.HasPrefix(source[i:], "{{"), strings.HasPrefix(source[i:], "}}"):
			literal.WriteByte(source[i])
			i += 2
		case strings.HasPrefix(source[i:], "{}"):
			flush()
			result = append(result, part{slot: true})
			i += 2
		case strings.HasPrefix(source[i:], "{:?}"):
			flush()
			result = append(result, part{slot: true, debug: true})
			i += 4
		case source[i] == '{' || source[i] == '}':
			return nil, fmt.Errorf("unsupported or unmatched format brace")
		default:
			literal.WriteByte(source[i])
			i++
		}
	}
	flush()
	return result, nil
}
func printTokens(ctx pm.Context, input token.TokenStream, newline bool) pm.Output {
	fail := func(message string) pm.Output { return pm.Failure(message, ctx.CallSpan()) }
	var arguments []token.TokenStream
	var current []token.Token
	for _, node := range input.Tokens() {
		if node.Kind() == token.SemicolonKind {
			return fail("format arguments cannot contain top-level semicolons")
		}
		if node.Kind() == token.PunctKind && node.Text() == "..." {
			return fail("variadic print arguments are unsupported")
		}
		if node.Kind() == token.PunctKind && node.Text() == "," {
			if len(current) == 0 {
				return fail("empty format argument")
			}
			arguments = append(arguments, token.New(current...))
			current = nil
			continue
		}
		current = append(current, node)
	}
	if len(current) > 0 {
		arguments = append(arguments, token.New(current...))
	}
	format := ""
	if len(arguments) == 0 {
		if !newline {
			return fail("print requires a format string literal")
		}
	} else {
		tokens := arguments[0].Tokens()
		for len(tokens) == 1 && tokens[0].Kind() == token.FragmentKind {
			tokens = tokens[0].Children().Tokens()
		}
		if len(tokens) != 1 || tokens[0].Kind() != token.LiteralKind || tokens[0].LiteralType() != token.String {
			return fail("print format must be a string literal")
		}
		var err error
		format, err = strconv.Unquote(tokens[0].Text())
		if err != nil {
			return fail(err.Error())
		}
		arguments = arguments[1:]
	}
	plan, err := parts(format)
	if err != nil {
		return fail(err.Error())
	}
	count := 0
	for _, part := range plan {
		if part.slot {
			count++
		}
	}
	if count != len(arguments) {
		return fail(fmt.Sprintf("format has %d placeholders but %d arguments", count, len(arguments)))
	}
	root, err := ctx.RuntimeRoot("github.com/goxide-lang/std")
	if err != nil {
		return fail(err.Error())
	}
	// Declarations keep this output in statement slots, and evaluate every
	// argument before running any formatter. No closure changes ? or recover.
	output := fragment(ctx, "var printingText string").Append(token.NewSemicolon(ctx.DefSite()))
	for i, argument := range arguments {
		output = output.Append(fragment(ctx, fmt.Sprintf("var printingArgument%d =", i)).Tokens()...).Append(argument.Tokens()...).Append(token.NewSemicolon(ctx.DefSite()))
	}
	index := 0
	for _, part := range plan {
		output = output.Append(fragment(ctx, "printingText +=").Tokens()...)
		if !part.slot {
			output = output.Append(fragment(ctx, strconv.Quote(part.literal)).Tokens()...)
		} else {
			member := "Display::display"
			if part.debug {
				member = "Debug::debug"
			}
			call := root.Append(fragment(ctx, "::"+member).Tokens()...).Append(group(ctx, token.Parenthesis, fragment(ctx, fmt.Sprintf("&printingArgument%d", index))))
			output = output.Append(call.Tokens()...)
			index++
		}
		output = output.Append(token.NewSemicolon(ctx.DefSite()))
	}
	if newline {
		output = output.Append(fragment(ctx, "printingText += \"\\n\"").Tokens()...).Append(token.NewSemicolon(ctx.DefSite()))
	}
	output = output.Append(root.Tokens()...).Append(fragment(ctx, "::formatting::Write(printingText)").Tokens()...).Append(token.NewSemicolon(ctx.DefSite()))
	return pm.Success(output)
}

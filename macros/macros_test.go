package std_macros

import (
	"strings"
	"testing"

	pm "github.com/goxide-lang/syntax/proc_macro"
	"github.com/goxide-lang/syntax/token"
)

func invocation(t *testing.T, source string, kind pm.Kind) (pm.Context, token.TokenStream) {
	t.Helper()
	scope := token.NewScope(t.Name())
	def, call, runtime := scope.NewContext(), scope.NewContext(), scope.NewContext()
	input, err := token.ParseText(source, call)
	if err != nil {
		t.Fatal(err)
	}
	if kind == pm.Function {
		nodes := input.Tokens()
		for len(nodes) > 0 && nodes[len(nodes)-1].Kind() == token.SemicolonKind {
			nodes = nodes[:len(nodes)-1]
		}
		input = token.New(nodes...)
	}
	root, err := token.NewIdent("canonical_std", false, runtime)
	if err != nil {
		t.Fatal(err)
	}
	config := pm.RequestConfig{Entry: "test", Kind: kind, Position: pm.ItemsPosition, DefSite: def, CallSite: call, Target: pm.Target{OS: "linux", Arch: "amd64"}, RuntimeRoots: map[string]token.TokenStream{"github.com/goxide-lang/std": token.New(root)}}
	if kind == pm.Derive {
		config.Item = input
	} else {
		config.Input = input
		config.Position = pm.StatementsPosition
	}
	request, err := pm.NewRequest(scope, config)
	if err != nil {
		t.Fatal(err)
	}
	if kind == pm.Derive {
		return request.Context(), request.Item()
	}
	return request.Context(), request.Input()
}

func TestDefaultOnlyConstrainsSelectedPayload(t *testing.T) {
	ctx, input := invocation(t, "enum Choice[T any,U any] { #[default] Selected(T), Inactive(U), }", pm.Derive)
	output := derive(ctx, input, "Default")
	if len(output.Diagnostics) != 0 {
		t.Fatal(output.Diagnostics)
	}
	printed := strings.ReplaceAll(output.Tokens.Display(), " ", "")
	if !strings.Contains(printed, "impl[Tany,Uany]canonical_std::DefaultforChoice[T,U]whereT:canonical_std::Default") || strings.Contains(printed, "whereU:") || strings.Contains(printed, ",U:canonical_std::Default") {
		t.Fatalf("selected payload constraints: %s", output.Tokens.Display())
	}
}

func TestDeriveReplaysRawPrivateFields(t *testing.T) {
	ctx, input := invocation(t, "struct Model[T any] where T: Bound { r#type T }", pm.Derive)
	output := derive(ctx, input, "Clone")
	if len(output.Diagnostics) != 0 {
		t.Fatal(output.Diagnostics)
	}
	count := 0
	var walk func(token.TokenStream)
	walk = func(stream token.TokenStream) {
		for _, node := range stream.Tokens() {
			if node.Kind() == token.IdentKind && node.Text() == "type" {
				count++
				if !node.Raw() || node.Context() != ctx.CallSite() {
					t.Fatalf("field identity changed: %+v", node)
				}
			}
			if node.Kind() == token.GroupKind {
				walk(node.Children())
			}
		}
	}
	walk(output.Tokens)
	if count != 2 {
		t.Fatalf("private field appears %d times, want constructor and projection", count)
	}
	text := strings.ReplaceAll(output.Tokens.Display(), " ", "")
	if !strings.Contains(text, "whereT:Bound,T:canonical_std::Clone") {
		t.Fatalf("original where clause not retained: %s", text)
	}
}

func TestDefaultRejectsAmbiguousSelection(t *testing.T) {
	for _, source := range []string{"enum E{A,B,}", "enum E{#[default] A,#[default] B,}", "enum E{#[default(1)] A,}"} {
		ctx, input := invocation(t, source, pm.Derive)
		output := derive(ctx, input, "Default")
		if len(output.Diagnostics) == 0 || !output.Tokens.IsEmpty() {
			t.Fatalf("invalid selection accepted: %s", source)
		}
	}
}

func TestFormatPlanRejectsMalformedSlots(t *testing.T) {
	for _, source := range []string{"{", "}", "{:x}", "{name}"} {
		if _, err := parts(source); err == nil {
			t.Errorf("accepted %q", source)
		}
	}
	plan, err := parts("{{}} {} {:?}")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 4 || plan[0].literal != "{} " || !plan[1].slot || plan[1].debug || !plan[3].debug {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestPrintEvaluatesArgumentsBeforeFormatting(t *testing.T) {
	ctx, input := invocation(t, `"{} {:?}", first(), second()`, pm.Function)
	output := printTokens(ctx, input, true)
	if len(output.Diagnostics) != 0 {
		t.Fatal(output.Diagnostics)
	}
	text := output.Tokens.Display()
	first, second, format := strings.Index(text, "first"), strings.Index(text, "second"), strings.Index(text, "display")
	if first < 0 || second <= first || format <= second {
		t.Fatalf("argument order changed: %s", text)
	}
	if strings.Contains(text, "func") {
		t.Fatalf("printing must retain the caller frame: %s", text)
	}
}

func TestErrorAttributesAndMessages(t *testing.T) {
	cases := []struct{ source, want string }{
		{`#[error("a")] #[error("b")] struct E {}`, "duplicate error"},
		{`#[error("{missing}")] struct E { value int }`, "unknown error placeholder"},
		{`#[error("a")] struct E { #[from] cause error; value int }`, "exactly one error field"},
		{`enum E { #[error("a")] #[from] A(error), #[error("b")] #[from] B(error), }`, "at most one from variant"},
		{`enum E { #[error("a")] #[source] A, }`, "one error payload"},
	}
	for _, tc := range cases {
		ctx, input := invocation(t, tc.source, pm.Derive)
		output := deriveError(ctx, input)
		if len(output.Diagnostics) == 0 || !strings.Contains(output.Diagnostics[0].Message, tc.want) {
			t.Errorf("%s: %+v", tc.source, output.Diagnostics)
		}
	}
	result, err := errorMessage("{{ {name} }}", map[string]string{"name": "field"})
	if err != nil || result != `"{ "+__runtime::error_scalar(field)+" }"` {
		t.Fatalf("message=%s err=%v", result, err)
	}
}

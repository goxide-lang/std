package typeinfo_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/goxide-lang/std/typeinfo"
)

type descriptor struct{}
type owner int
type ownerAlias = owner

var calls int

func (owner) GoxideInternalTypeCarrierForSelf(owner) descriptor {
	calls++
	panic("carrier method must not run")
}

type pointerOwner struct{}

func (*pointerOwner) GoxideInternalTypeCarrierForSelf(*pointerOwner) descriptor {
	panic("carrier method must not run, including on nil receivers")
}

func TestOfReturnsNamedZeroWithoutCallingHelper(t *testing.T) {
	if typeinfo.GoxideTypeInfoABI != 1 {
		t.Fatal("unexpected carrier ABI")
	}
	var explicit descriptor = typeinfo.Of[owner, descriptor]()
	var inferred descriptor = typeinfo.Of[owner]()
	var alias descriptor = typeinfo.Of[ownerAlias]()
	var pointer descriptor = typeinfo.Of[*pointerOwner]()
	if explicit != (descriptor{}) || inferred != explicit || alias != explicit || pointer != explicit || calls != 0 {
		t.Fatal("wrong carrier identity, zero value, or helper invocation")
	}
}

// Load the real public declaration before checking consumers. These cases
// exercise ordinary Go constraints; they do not claim frontend acceptance.
func TestCarrierConstraintRejectsWrongIdentities(t *testing.T) {
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, "typeinfo.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := (&types.Config{}).Check("github.com/goxide-lang/std/typeinfo", fs, []*ast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, declarations, call, want string
	}{
		{"wrong-self", "type Other int; func (Owner) GoxideInternalTypeCarrierForSelf(Other) Descriptor {panic(0)}", "info.Of[Owner]()", "GoxideInternalTypeCarrierForSelf"},
		{"promoted-parent", "type Parent struct{}; type Child struct{Parent}; func (Parent) GoxideInternalTypeCarrierForSelf(Parent) Descriptor {panic(0)}", "info.Of[Child]()", "GoxideInternalTypeCarrierForSelf"},
		{"different-method", "func (Owner) TypeCarrierForSelf(Owner) Descriptor {panic(0)}", "info.Of[Owner]()", "GoxideInternalTypeCarrierForSelf"},
		{"nonempty-carrier", "type Nonempty struct{Value int}; func (Owner) GoxideInternalTypeCarrierForSelf(Owner) Nonempty {panic(0)}", "info.Of[Owner,Nonempty]()", "~struct{}"},
		{"interface-carrier", "func (Owner) GoxideInternalTypeCarrierForSelf(Owner) any {panic(0)}", "info.Of[Owner,any]()", "~struct{}"},
		{"wrong-carrier-identity", "type OtherDescriptor struct{}; func (Owner) GoxideInternalTypeCarrierForSelf(Owner) Descriptor {panic(0)}", "info.Of[Owner,OtherDescriptor]()", "GoxideInternalTypeCarrierForSelf"},
		{"pointer-method-set", "func (*Owner) GoxideInternalTypeCarrierForSelf(*Owner) Descriptor {panic(0)}", "info.Of[Owner]()", "GoxideInternalTypeCarrierForSelf"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "package consumer\nimport info \"github.com/goxide-lang/std/typeinfo\"\ntype Owner int; type Descriptor struct{}\n" + tc.declarations + "\nvar _ = " + tc.call
			fs := token.NewFileSet()
			file, err := parser.ParseFile(fs, "consumer.go", source, 0)
			if err != nil {
				t.Fatal(err)
			}
			_, err = (&types.Config{Importer: carrierImporter{pkg}}).Check("consumer", fs, []*ast.File{file}, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected constraint rejection containing %q; got %v", tc.want, err)
			}
		})
	}
}

type carrierImporter struct{ pkg *types.Package }

func (i carrierImporter) Import(path string) (*types.Package, error) {
	if path != i.pkg.Path() {
		panic("unexpected fixture import: " + path)
	}
	return i.pkg, nil
}

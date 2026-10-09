package goxideruntime_test

import (
	"testing"

	goxideruntime "github.com/goxide-lang/std/runtime"
)

func TestAssignmentGuard(t *testing.T) {
	propagate := func() (result string) {
		guard := goxideruntime.Assignment[string]{}
		defer guard.Catch(&result)
		guard.Fail("propagated")
		return "unreachable"
	}
	if got := propagate(); got != "propagated" {
		t.Fatalf("result = %q", got)
	}

	marker := new(int)
	func() {
		defer func() {
			if got := recover(); got != marker {
				t.Fatalf("ordinary panic = %v, want original marker", got)
			}
		}()
		var result string
		guard := goxideruntime.Assignment[string]{}
		defer guard.Catch(&result)
		panic(marker)
	}()
}

// A caller's type named comparable must not capture the Go constraint used by
// generated map/propagation helpers.
type comparable struct{}

func lookup[T goxideruntime.Comparable](values map[T]string, key T) string {
	return values[key]
}

func TestComparableConstraintIgnoresCallerBinding(t *testing.T) {
	if got := lookup(map[int]string{3: "value"}, 3); got != "value" {
		t.Fatalf("lookup = %q", got)
	}
}

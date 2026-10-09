package sum_test

import (
	"github.com/goxide-lang/std/sum"
	"testing"
)

type alias[T, E any] = sum.Result[T, E]
type failure struct{}

func (*failure) Error() string { return "failure" }

func rejects(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("invalid operation accepted")
		}
	}()
	f()
}

func TestValueSemantics(t *testing.T) {
	var r alias[int, error] = sum.NewOk[int, error](7)
	if sum.ValueOk(r) != 7 {
		t.Fatal(r)
	}
	var absent sum.Option[int]
	if sum.TagOption(absent) != 0 || sum.TagOption(sum.None[int]()) != 0 {
		t.Fatal("zero Option is not None")
	}
	present := sum.NewSome((*int)(nil))
	if sum.TagOption(present) != 1 || sum.ValueSome(present) != nil {
		t.Fatal("Some(nil) collapsed")
	}
	nested := sum.NewSome(sum.None[int]())
	if sum.TagOption(nested) != 1 || sum.TagOption(sum.ValueSome(nested)) != 0 {
		t.Fatal("Some(None) collapsed")
	}
	values := []int{1}
	copied := sum.NewSome(values)
	values[0] = 2
	if sum.ValueSome(copied)[0] != 2 {
		t.Fatal("assignment unexpectedly cloned")
	}
	nilErr := sum.NewErr[int, error](nil)
	if sum.ValueErr(nilErr) != nil {
		t.Fatal("nil Err payload changed")
	}
	var ptr *failure
	boxed := sum.NewErr[int, error](ptr)
	if sum.ValueErr(boxed) == nil {
		t.Fatal("typed nil interface changed")
	}
	partial := struct {
		Value int
		Err   *failure
	}{3, &failure{}}
	kept := sum.NewErr[int, struct {
		Value int
		Err   *failure
	}](partial)
	if sum.ValueErr(kept) != partial {
		t.Fatal("partial value lost")
	}
	// Shape checks intentionally do not recursively validate a payload.
	sum.ValidateOption(sum.NewSome(sum.Result[int, error]{}))
}

func TestResultFixedSlotsAndCopies(t *testing.T) {
	r := sum.NewOk[int, int](1)
	before := r
	p := sum.ProjectOk(&r)
	*p = 2
	after := r
	*p = 3
	if sum.ValueOk(before) != 1 || sum.ValueOk(after) != 2 || sum.ValueOk(r) != 3 {
		t.Fatal("copy aliases scalar storage")
	}
	r = sum.NewOk[int, int](4)
	if *p != 4 || p != sum.ProjectOk(&r) {
		t.Fatal("same variant replacement changed slot")
	}
	r = sum.NewErr[int, int](6)
	q := sum.ProjectErr(&r)
	if *p != 0 || p == q {
		t.Fatal("variant slots overlap")
	}
	*p = 9
	sum.ValidateResult(r)
	if sum.ValueErr(r) != 6 {
		t.Fatal("inactive write changed active payload")
	}
	r = sum.NewOk[int, int](5)
	if *p != 5 || *q != 0 {
		t.Fatal("replacement did not overwrite whole value")
	}
}

func TestOptionFixedSlotAndEscapes(t *testing.T) {
	makeRef := func() (*int, func() int) {
		v := sum.NewSome(8)
		p := sum.ProjectSome(&v)
		return p, func() int { return sum.ValueSome(v) }
	}
	p, read := makeRef()
	saved := p
	*saved = 9
	if read() != 9 {
		t.Fatal("returned/captured projection detached")
	}
	v := sum.NewSome([]int{1})
	copy := v
	slot := sum.ProjectSome(&v)
	(*slot)[0] = 2
	if sum.ValueSome(copy)[0] != 2 {
		t.Fatal("slice contents unexpectedly cloned")
	}
	*slot = []int{3}
	if sum.ValueSome(copy)[0] != 2 {
		t.Fatal("slice header copies share slots")
	}
	v = sum.None[[]int]()
	if *slot != nil {
		t.Fatal("None did not clear previous payload")
	}
	*slot = []int{7}
	sum.ValidateOption(v)
	if sum.TagOption(v) != 0 {
		t.Fatal("inactive write activated Some")
	}
	v = sum.NewSome([]int{4})
	if (*slot)[0] != 4 {
		t.Fatal("old projection lost fixed slot")
	}
}

func TestInvalidOperations(t *testing.T) {
	for name, f := range map[string]func(){
		"zero-result":       func() { sum.ValidateResult(sum.Result[int, error]{}) },
		"none-value":        func() { sum.ValueSome(sum.None[int]()) },
		"none-project":      func() { v := sum.None[int](); sum.ProjectSome(&v) },
		"nil-option":        func() { sum.ProjectSome[int](nil) },
		"wrong-ok-value":    func() { sum.ValueOk(sum.NewErr[int, int](2)) },
		"wrong-err-value":   func() { sum.ValueErr(sum.NewOk[int, int](1)) },
		"wrong-ok-project":  func() { v := sum.NewErr[int, int](2); sum.ProjectOk(&v) },
		"wrong-err-project": func() { v := sum.NewOk[int, int](1); sum.ProjectErr(&v) },
		"nil-ok":            func() { sum.ProjectOk[int, int](nil) },
		"nil-err":           func() { sum.ProjectErr[int, int](nil) },
	} {
		t.Run(name, func(t *testing.T) { rejects(t, f) })
	}
}

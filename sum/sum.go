// Package sum provides shared value representations for Goxide Option and Result.
// Shape validation does not replace compiler-generated payload validation.
package sum

// GoxideSumABI identifies the representation contract, not a module release.
const GoxideSumABI = 3

// Tag names Go's int type for generated discriminants even when a source
// package declares its own int. It does not change the runtime representation.
type Tag = int

// Option has an independent inline Some slot. Its zero value is None.
// Assignment copies the slot with ordinary Go shallow-value semantics.
type Option[T any] struct {
	tag  uint8
	some T
}

// NewSome constructs Some without cloning or validating its payload.
func NewSome[T any](value T) Option[T] { return Option[T]{tag: 1, some: value} }

// None constructs an absent Option. Some with a nil payload remains present.
func None[T any]() Option[T] { return Option[T]{} }

// ValidateOption checks the tag, not inactive storage or payload invariants.
func ValidateOption[T any](value Option[T]) {
	if value.tag > 1 {
		panic("goxide: invalid enum Option")
	}
}

// TagOption reads the raw tag: 0 denotes None and 1 denotes Some.
// It does not replace ValidateOption.
func TagOption[T any](value Option[T]) int { return int(value.tag) }

// ValueSome copies the active payload using ordinary Go assignment.
func ValueSome[T any](value Option[T]) T {
	if TagOption(value) != 1 {
		panic("goxide: expected Option::Some")
	}
	return value.some
}

// ProjectSome returns the actual Some slot. The pointer remains attached to
// this storage even if the Option is later replaced with None or another Some.
// It is an ordinary Go pointer; no exclusivity or lifetime restriction is added.
func ProjectSome[T any](value *Option[T]) *T {
	if value == nil {
		panic("goxide: nil Option projection")
	}
	if TagOption(*value) != 1 {
		panic("goxide: expected Option::Some")
	}
	return &value.some
}

// Result has independent inline Ok and Err slots. Its zero value is invalid.
// Both type parameters participate in its nominal identity and storage.
type Result[T, E any] struct {
	tag uint8
	ok  T
	err E
}

// NewOk constructs Ok without cloning or validating its payload.
func NewOk[T, E any](value T) Result[T, E] { return Result[T, E]{tag: 1, ok: value} }

// NewErr constructs Err even when the payload is nil. Whether a native Go error
// indicates success belongs to the call adapter, not this constructor.
func NewErr[T, E any](value E) Result[T, E] { return Result[T, E]{tag: 2, err: value} }

// ValidateResult checks only the active tag. Inactive slots may contain values
// written through older projections; they are not part of the active variant.
func ValidateResult[T, E any](value Result[T, E]) {
	if value.tag != 1 && value.tag != 2 {
		panic("goxide: invalid enum Result")
	}
}

// TagResult reads the raw tag: 1 denotes Ok and 2 denotes Err; zero is invalid.
// It does not replace ValidateResult.
func TagResult[T, E any](value Result[T, E]) int { return int(value.tag) }

// ValueOk copies the active Ok payload.
func ValueOk[T, E any](value Result[T, E]) T {
	if TagResult(value) != 1 {
		panic("goxide: expected Result::Ok")
	}
	return value.ok
}

// ValueErr copies the active Err payload.
func ValueErr[T, E any](value Result[T, E]) E {
	if TagResult(value) != 2 {
		panic("goxide: expected Result::Err")
	}
	return value.err
}

// ProjectOk returns the actual Ok slot, with the same fixed-storage contract as ProjectSome.
func ProjectOk[T, E any](value *Result[T, E]) *T {
	if value == nil {
		panic("goxide: nil Result projection")
	}
	if TagResult(*value) != 1 {
		panic("goxide: expected Result::Ok")
	}
	return &value.ok
}

// ProjectErr returns a separate slot even when T and E are the same type.
func ProjectErr[T, E any](value *Result[T, E]) *E {
	if value == nil {
		panic("goxide: nil Result projection")
	}
	if TagResult(*value) != 2 {
		panic("goxide: expected Result::Err")
	}
	return &value.err
}

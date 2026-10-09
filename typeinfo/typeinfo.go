// Package typeinfo supplies the compile-time carrier constraint used by
// Goxide-generated Go. It does not discover types or call carrier methods.
package typeinfo

// GoxideTypeInfoABI identifies the carrier contract, not a module release.
const GoxideTypeInfoABI = 1

// CarrierFor associates T with a zero-field struct type S. The method's T
// parameter requires the exact self type: promoting a method from an embedded
// parent does not associate its child with the parent's carrier.
//
// This structural Go constraint does not establish compiler provenance. The
// compiler must separately validate any generated metadata it trusts.
type CarrierFor[T any, S ~struct{}] interface {
	GoxideInternalTypeCarrierForSelf(T) S
}

// Of returns the zero value of T's carrier type. It does not construct T, call
// its carrier method, or inspect a value at runtime.
func Of[T CarrierFor[T, S], S ~struct{}]() S {
	var zero S
	return zero
}

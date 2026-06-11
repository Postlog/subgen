package utils

// DereferenceOrNil maps an optional value to a database/sql parameter: a nil pointer
// becomes NULL, otherwise the pointed-to value (as any). database/sql already
// dereferences pointers itself, but spelling it out keeps the nil→NULL intent explicit
// at the call site instead of relying on the driver's implicit behaviour.
//
// It does NOT do value conversion — for a bool stored in an INTEGER column, see
// boolIntPtr (which additionally maps true/false → 1/0); here T flows through as-is.
func DereferenceOrNil[T any](p *T) any {
	if p == nil {
		return nil
	}

	return *p
}

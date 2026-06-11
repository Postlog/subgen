// Package utils holds tiny, dependency-free generic helpers shared across packages.
package utils

// Ptr returns a pointer to v. Handy for the optional/nullable fields that are *T —
// inline-able where a literal can't be addressed (e.g. Ptr("note"), Ptr(int64(1))).
func Ptr[T any](v T) *T { return &v }

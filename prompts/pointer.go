package prompts

// Returns the value of the pointer type T. Will panic if v is nil.
func fromPointer[T any](v *T) T {
	return *v
}

// Returns a pointer to the value of type T
func toPointer[T any](v T) *T {
	return &v
}

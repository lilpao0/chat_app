package user

// UpdateField preserves whether a PATCH field was omitted. When Set is true,
// a nil Value represents an explicit JSON null at the application boundary.
type UpdateField[T any] struct {
	Set   bool
	Value *T
}

// UpdateProfileInput describes requested changes without depending on JSON or
// HTTP types. Validation and date parsing belong to the update use case.
type UpdateProfileInput struct {
	FirstName   UpdateField[string]
	LastName    UpdateField[string]
	DateOfBirth UpdateField[string]
	PhoneNumber UpdateField[string]
}

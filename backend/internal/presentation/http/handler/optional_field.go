package handler

import (
	"bytes"
	"encoding/json"
)

// optionalJSONField distinguishes an omitted PATCH field from explicit null
// and from a concrete value. Business validation happens after decoding.
type optionalJSONField[T any] struct {
	Set   bool
	Null  bool
	Value T
}

func (f *optionalJSONField[T]) UnmarshalJSON(data []byte) error {
	f.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		f.Null = true
		var zero T
		f.Value = zero
		return nil
	}
	f.Null = false
	return json.Unmarshal(data, &f.Value)
}

type updateProfileRequest struct {
	FirstName   optionalJSONField[string] `json:"first_name"`
	LastName    optionalJSONField[string] `json:"last_name"`
	DateOfBirth optionalJSONField[string] `json:"date_of_birth"`
	PhoneNumber optionalJSONField[string] `json:"phone_number"`
}

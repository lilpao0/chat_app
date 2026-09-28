package handler

import (
	"encoding/json"
	"testing"
)

func TestUpdateProfileRequestPresence(t *testing.T) {
	t.Run("omitted fields remain unset", func(t *testing.T) {
		var request updateProfileRequest
		if err := json.Unmarshal([]byte(`{}`), &request); err != nil {
			t.Fatal(err)
		}
		for name, field := range profileRequestFields(request) {
			if field.Set {
				t.Errorf("%s is set, want omitted", name)
			}
		}
	})

	t.Run("explicit null is preserved", func(t *testing.T) {
		var request updateProfileRequest
		body := []byte(`{
			"first_name": null,
			"last_name": null,
			"date_of_birth": null,
			"phone_number": null
		}`)
		if err := json.Unmarshal(body, &request); err != nil {
			t.Fatal(err)
		}
		for name, field := range profileRequestFields(request) {
			if !field.Set || !field.Null {
				t.Errorf("%s = %+v, want set explicit null", name, field)
			}
		}
	})

	t.Run("concrete values are preserved", func(t *testing.T) {
		var request updateProfileRequest
		body := []byte(`{
			"first_name": "Pao",
			"last_name": "Nguyen",
			"date_of_birth": "2002-05-21",
			"phone_number": "+84901234567"
		}`)
		if err := json.Unmarshal(body, &request); err != nil {
			t.Fatal(err)
		}
		want := map[string]string{
			"first_name": "Pao", "last_name": "Nguyen",
			"date_of_birth": "2002-05-21", "phone_number": "+84901234567",
		}
		for name, field := range profileRequestFields(request) {
			if !field.Set || field.Null || field.Value != want[name] {
				t.Errorf("%s = %+v, want value %q", name, field, want[name])
			}
		}
	})
}

func TestOptionalJSONFieldRejectsWrongType(t *testing.T) {
	var request updateProfileRequest
	if err := json.Unmarshal([]byte(`{"phone_number":123}`), &request); err == nil {
		t.Fatal("numeric phone_number was accepted")
	}
}

func profileRequestFields(request updateProfileRequest) map[string]optionalJSONField[string] {
	return map[string]optionalJSONField[string]{
		"first_name": request.FirstName, "last_name": request.LastName,
		"date_of_birth": request.DateOfBirth, "phone_number": request.PhoneNumber,
	}
}

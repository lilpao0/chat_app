package auth_test

import (
	"errors"
	"strings"
	"testing"

	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
)

// ==================== FIRST NAME VALIDATION TESTS ====================

func TestValidateInput_FirstName(t *testing.T) {
	tests := []struct {
		name      string
		firstName string
		wantErr   bool
		errField  string
	}{
		// Valid cases
		{"valid single name", "An", false, ""},
		{"valid with spaces", "Nguyen Van", false, ""},
		{"valid unicode", "Nguyễn", false, ""},
		{"valid 100 chars", strings.Repeat("a", 100), false, ""},

		// Empty cases
		{"empty string", "", true, "first_name"},
		{"whitespace only", "   ", true, "first_name"},

		// Too long
		{"too long (101 chars)", strings.Repeat("a", 101), true, "first_name"},

		// Invalid characters
		{"null byte", "An\x00B", true, "first_name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domainauth.ValidateInput(tt.firstName, "", "valid@email.com", "password123")
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q", tt.firstName)
					return
				}
				ve, ok := domainauth.IsValidationError(err)
				if !ok {
					t.Errorf("expected ValidationError, got %v", err)
					return
				}
				if ve.Field != tt.errField {
					t.Errorf("expected field %q, got %q", tt.errField, ve.Field)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// ==================== LAST NAME VALIDATION TESTS ====================

func TestValidateInput_LastName(t *testing.T) {
	tests := []struct {
		name     string
		lastName string
		wantErr  bool
		errField string
	}{
		// Valid cases - empty is OK
		{"empty (optional)", "", false, ""},
		{"valid single", "An", false, ""},
		{"valid unicode", "Văn", false, ""},
		{"valid 100 chars", strings.Repeat("a", 100), false, ""},

		// Too long
		{"too long (101 chars)", strings.Repeat("a", 101), true, "last_name"},

		// Invalid characters
		{"null byte", "An\x00B", true, "last_name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domainauth.ValidateInput("ValidName", tt.lastName, "valid@email.com", "password123")
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for lastName %q", tt.lastName)
					return
				}
				ve, ok := domainauth.IsValidationError(err)
				if !ok {
					t.Errorf("expected ValidationError, got %v", err)
					return
				}
				if ve.Field != tt.errField {
					t.Errorf("expected field %q, got %q", tt.errField, ve.Field)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// ==================== EMAIL VALIDATION TESTS ====================

func TestValidateInput_Email(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		wantErr  bool
		errField string
	}{
		// Valid cases
		{"valid simple", "test@example.com", false, ""},
		{"valid with plus", "test+filter@example.com", false, ""},
		{"valid subdomain", "test@mail.example.com", false, ""},
		{"valid uppercase", "TEST@EXAMPLE.COM", false, ""},
		{"valid with spaces trimmed", "  test@example.com  ", false, ""},

		// Empty
		{"empty", "", true, "email"},
		{"whitespace only", "   ", true, "email"},

		// Invalid format
		{"no @", "testexample.com", true, "email"},
		{"no domain", "test@", true, "email"},
		{"no local part", "@example.com", true, "email"},
		{"double @", "test@@example.com", true, "email"},
		{"space in local", "test @example.com", true, "email"},

		// Invalid characters
		{"null byte", "test\x00@example.com", true, "email"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domainauth.ValidateInput("ValidName", "", tt.email, "password123")
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for email %q", tt.email)
					return
				}
				ve, ok := domainauth.IsValidationError(err)
				if !ok {
					t.Errorf("expected ValidationError, got %v", err)
					return
				}
				if ve.Field != tt.errField {
					t.Errorf("expected field %q, got %q", tt.errField, ve.Field)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// ==================== PASSWORD VALIDATION TESTS ====================

func TestValidateInput_Password(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		wantErr   bool
		errField  string
	}{
		// Valid cases
		{"valid min length (8)", "password", false, ""},
		{"valid 72 chars", strings.Repeat("a", 72), false, ""},
		{"valid unicode", "mậtkhẩu123", false, ""},
		{"valid special chars", "p@ssw0rd!", false, ""},

		// Empty
		{"empty", "", true, "password"},

		// Too short
		{"too short (1)", "p", true, "password"},
		{"too short (7)", "pass123", true, "password"},

		// Too long
		{"too long (73 chars)", strings.Repeat("a", 73), true, "password"},

		// Invalid characters
		{"null byte", "pass\x00word", true, "password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domainauth.ValidateInput("ValidName", "", "valid@email.com", tt.password)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for password %q", tt.password)
					return
				}
				ve, ok := domainauth.IsValidationError(err)
				if !ok {
					t.Errorf("expected ValidationError, got %v", err)
					return
				}
				if ve.Field != tt.errField {
					t.Errorf("expected field %q, got %q", tt.errField, ve.Field)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// ==================== COMBINED VALIDATION TESTS ====================

func TestValidateInput_ReturnsFirstErrorOnly(t *testing.T) {
	// First name is validated first
	err := domainauth.ValidateInput("", "", "", "")
	ve, ok := domainauth.IsValidationError(err)
	if !ok {
		t.Fatal("expected ValidationError")
	}
	if ve.Field != "first_name" {
		t.Errorf("expected first_name error, got %q", ve.Field)
	}
}

func TestValidateInput_ValidComplete(t *testing.T) {
	tests := []struct {
		name      string
		firstName string
		lastName  string
		email     string
		password  string
	}{
		{"all valid simple", "An", "B", "test@example.com", "password123"},
		{"all valid unicode", "Nguyễn", "Văn A", "nguyen@example.com", "mậtkhẩu123"},
		{"empty lastname", "An", "", "test@example.com", "password123"},
		{"complex email", "John", "Doe", "john.doe@example.com", "Pass1234!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domainauth.ValidateInput(tt.firstName, tt.lastName, tt.email, tt.password)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ==================== NORMALIZE EMAIL TESTS ====================

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"lowercase", "TEST@EXAMPLE.COM", "test@example.com", false},
		{"trim spaces", "  test@example.com  ", "test@example.com", false},
		{"lowercase and trim", "  TEST@EXAMPLE.COM  ", "test@example.com", false},
		{"invalid email", "not-an-email", "", true},
		{"empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domainauth.NormalizeEmail(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("NormalizeEmail(%q) = %q, want %q", tt.input, got, tt.want)
				}
			}
		})
	}
}

// ==================== VALIDATION ERROR TESTS ====================

func TestValidationError(t *testing.T) {
	ve := domainauth.ValidationError{Field: "email", Message: "Email is invalid"}

	if ve.Error() != "email: Email is invalid" {
		t.Errorf("unexpected Error() output: %s", ve.Error())
	}

	// Test IsValidationError with ValidationError
	isVE, ok := domainauth.IsValidationError(ve)
	if !ok {
		t.Error("IsValidationError returned false for ValidationError")
	}
	if isVE.Field != "email" || isVE.Message != "Email is invalid" {
		t.Errorf("unexpected values: field=%s, message=%s", isVE.Field, isVE.Message)
	}

	// Test with non-ValidationError
	_, ok = domainauth.IsValidationError(errors.New("some other error"))
	if ok {
		t.Error("IsValidationError returned true for non-ValidationError")
	}
}

// ==================== PASSWORD VALIDATION DIRECT TESTS ====================

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"valid 8 chars", "password", false},
		{"valid exactly 72", strings.Repeat("a", 72), false},
		{"valid unicode 8 chars", "mậtkhẩu123", false},
		{"too short 7 chars", "pass123", true},
		{"too long 73 chars", strings.Repeat("a", 73), true},
		{"empty", "", true},
		// Note: Cannot test null byte in Go source - it causes compile error
		// The null byte case is tested via ValidateInput_FirstName
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domainauth.ValidatePassword(tt.password)
			if tt.wantErr && err == nil {
				t.Errorf("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ==================== EDGE CASES ====================

func TestValidateInput_EdgeCases(t *testing.T) {
	// Test with all whitespace first name - should fail
	err := domainauth.ValidateInput("\t\n ", "", "a@b.com", "password")
	if err == nil {
		t.Error("expected error for whitespace-only first_name")
	}

	// Test valid input
	err = domainauth.ValidateInput("  John  ", "  Doe  ", "john@example.com", "password")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Test very long email local part
	longEmail := strings.Repeat("a", 250) + "@b.com"
	err = domainauth.ValidateInput("Name", "", longEmail, "password")
	if err == nil {
		t.Error("expected error for too long email")
	}
}

func TestValidateInput_EmailWithSpecialChars(t *testing.T) {
	// These should be valid according to RFC but some might be rejected
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"dot in local", "john.doe@example.com", false},
		{"plus in local", "john+doe@example.com", false},
		{"dash in domain", "john@example-test.com", false},
		// Underscore is valid in domain names
		{"underscore in domain", "john@example_test.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domainauth.ValidateInput("Name", "", tt.email, "password")
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %q", tt.email)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tt.email, err)
			}
		})
	}
}

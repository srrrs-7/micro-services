package request

import (
	"strings"
	"testing"
)

func TestLoginRequest_Validate(t *testing.T) {
	cases := []struct {
		name    string
		req     LoginRequest
		wantErr bool
	}{
		// happy path
		{"valid", LoginRequest{Email: "alice@example.com", Password: "pw1234"}, false},

		// email: presence + length boundaries
		{"empty email", LoginRequest{Email: "", Password: "pw1234"}, true},
		{"email at minimum length (5 chars)", LoginRequest{Email: "a@b.c", Password: "pw1234"}, false},
		{"email too short (4 chars)", LoginRequest{Email: "a@b.", Password: "pw1234"}, true},
		{"email at maximum length (100 chars)",
			LoginRequest{Email: strings.Repeat("a", 88) + "@example.com", Password: "pw1234"}, false},
		{"email too long (101 chars)",
			LoginRequest{Email: strings.Repeat("a", 89) + "@example.com", Password: "pw1234"}, true},

		// email: format
		{"email missing @", LoginRequest{Email: "alice.example.com", Password: "pw1234"}, true},
		{"email missing local part", LoginRequest{Email: "@example.com", Password: "pw1234"}, true},
		{"email missing domain", LoginRequest{Email: "alice@", Password: "pw1234"}, true},
		{"email missing TLD", LoginRequest{Email: "alice@example", Password: "pw1234"}, true},
		{"email with embedded space", LoginRequest{Email: "ali ce@example.com", Password: "pw1234"}, true},
		{"email with leading space", LoginRequest{Email: " alice@example.com", Password: "pw1234"}, true},
		{"email with trailing space", LoginRequest{Email: "alice@example.com ", Password: "pw1234"}, true},

		// password: presence + length boundaries
		{"empty password", LoginRequest{Email: "alice@example.com", Password: ""}, true},
		{"password too short (5 chars)", LoginRequest{Email: "alice@example.com", Password: "pw123"}, true},
		{"password at minimum length (6 chars)", LoginRequest{Email: "alice@example.com", Password: "pw1234"}, false},
		{"password at maximum length (100 chars)",
			LoginRequest{Email: "alice@example.com", Password: strings.Repeat("p", 100)}, false},
		{"password too long (101 chars)",
			LoginRequest{Email: "alice@example.com", Password: strings.Repeat("p", 101)}, true},

		// both fields missing
		{"both empty", LoginRequest{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

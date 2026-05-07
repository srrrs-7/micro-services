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
		{"valid", LoginRequest{Email: "a@b.co", Password: "pw1234"}, false},
		{"empty email", LoginRequest{Email: "", Password: "pw1234"}, true},
		{"email too short (4 chars)", LoginRequest{Email: "a@b.", Password: "pw1234"}, true},
		{"email at minimum length (5 chars)", LoginRequest{Email: "a@b.c", Password: "pw1234"}, false},
		{"email too long (101 chars)", LoginRequest{Email: strings.Repeat("a", 95) + "@b.com", Password: "pw1234"}, true},
		{"empty password", LoginRequest{Email: "a@b.co", Password: ""}, true},
		{"password too short (5 chars)", LoginRequest{Email: "a@b.co", Password: "pw123"}, true},
		{"password at minimum length (6 chars)", LoginRequest{Email: "a@b.co", Password: "pw1234"}, false},
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

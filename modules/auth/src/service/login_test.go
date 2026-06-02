package service

import (
	"auth/domain"
	"auth/infra/database/db"
	"context"
	"errors"
	"shared/utilhttp"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// tokenCmpOpts handles the domain.Expired (time.Time) field, which has
// unexported fields cmp cannot descend into without help.
var tokenCmpOpts = cmp.Options{
	cmp.Transformer("expiredAsTime", func(e domain.Expired) time.Time { return time.Time(e) }),
	cmpopts.EquateApproxTime(time.Second),
}

// stubQuerier embeds db.Querier so unimplemented methods nil-panic loudly.
// Override only what the test under exercise needs.
type stubQuerier struct {
	db.Querier
	getUserFn func(ctx context.Context, email string) (db.User, error)
}

func (s stubQuerier) GetUser(ctx context.Context, email string) (db.User, error) {
	return s.getUserFn(ctx, email)
}

func TestLoginService_Post_returnsTokenForValidCredentials(t *testing.T) {
	const (
		inputEmail = "alice@example.com"
		password   = "pw1234"
	)

	var capturedEmail string
	repo := stubQuerier{
		getUserFn: func(_ context.Context, email string) (db.User, error) {
			capturedEmail = email
			return db.User{
				ID:       1,
				LoginID:  "alice",
				Email:    email,
				Password: password,
			}, nil
		},
	}

	got, err := NewLoginService(repo, nil).Post(
		context.Background(),
		domain.NewLoginInput(inputEmail, password),
	)
	if err != nil {
		t.Fatalf("Post() unexpected error: %v", err)
	}

	if capturedEmail != inputEmail {
		t.Errorf("repo.GetUser called with email = %q, want %q", capturedEmail, inputEmail)
	}

	// Pin the current contract: the service returns an empty *domain.Token.
	// When token issuance is implemented, this expectation must be updated.
	want := &domain.Token{}
	if diff := cmp.Diff(want, got, tokenCmpOpts); diff != "" {
		t.Errorf("returned token mismatch (-want +got):\n%s", diff)
	}
}

func TestLoginService_Post_returnsUnauthorizedOnPasswordMismatch(t *testing.T) {
	repo := stubQuerier{
		getUserFn: func(context.Context, string) (db.User, error) {
			return db.User{Email: "alice@example.com", Password: "stored_pw"}, nil
		},
	}

	got, err := NewLoginService(repo, nil).Post(
		context.Background(),
		domain.NewLoginInput("alice@example.com", "wrong_pw"),
	)
	if got != nil {
		t.Errorf("expected nil token on auth failure, got %+v", got)
	}

	var unauth utilhttp.UnauthorizedError
	if !errors.As(err, &unauth) {
		t.Fatalf("expected utilhttp.UnauthorizedError, got %T: %v", err, err)
	}
	if unauth.Type != utilhttp.ErrUnauthorized {
		t.Errorf("error type = %v, want %v", unauth.Type, utilhttp.ErrUnauthorized)
	}
}

func TestLoginService_Post_wrapsRepoErrorAsDBError(t *testing.T) {
	sentinel := errors.New("connection refused")
	repo := stubQuerier{
		getUserFn: func(context.Context, string) (db.User, error) {
			return db.User{}, sentinel
		},
	}

	got, err := NewLoginService(repo, nil).Post(
		context.Background(),
		domain.NewLoginInput("alice@example.com", "pw1234"),
	)
	if got != nil {
		t.Errorf("expected nil token when repo errors, got %+v", got)
	}

	var dbErr utilhttp.DBError
	if !errors.As(err, &dbErr) {
		t.Fatalf("expected utilhttp.DBError, got %T: %v", err, err)
	}
	if dbErr.Type != utilhttp.ErrDatabase {
		t.Errorf("error type = %v, want %v", dbErr.Type, utilhttp.ErrDatabase)
	}
	// The wrapped repo error is formatted with %v, so the sentinel is not
	// chain-unwrappable. Verify only that the message includes its text.
	if got := dbErr.Error(); !contains(got, sentinel.Error()) {
		t.Errorf("DBError message %q does not contain sentinel %q", got, sentinel.Error())
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

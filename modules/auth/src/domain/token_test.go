package domain

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// expiredAsTime treats the named-type Expired as time.Time for cmp comparison,
// so EquateApproxTime can match across the small skew between the test's
// time.Now() and the constructor's internal time.Now().
var expiredAsTime = cmp.Transformer("expiredAsTime", func(e Expired) time.Time {
	return time.Time(e)
})

func TestNewAccessToken_populatesFieldsAndExpiresIn15Minutes(t *testing.T) {
	var (
		uid   UserID = "user-123"
		scope Scope  = "read"
		role  Role   = "admin"
	)

	before := time.Now()
	got := NewAccessToken(uid, scope, role)

	want := AccessToken{
		UserID:  uid,
		Scope:   scope,
		Role:    role,
		Expired: Expired(before.Add(15 * time.Minute)),
	}

	opts := cmp.Options{
		expiredAsTime,
		cmpopts.EquateApproxTime(time.Second),
	}
	if diff := cmp.Diff(want, got, opts); diff != "" {
		t.Errorf("NewAccessToken() mismatch (-want +got):\n%s", diff)
	}
}

func TestNewRefreshToken_populatesFieldsAndExpiresIn12Hours(t *testing.T) {
	var (
		uid   UserID = "user-456"
		scope Scope  = "write"
		role  Role   = "user"
	)

	before := time.Now()
	got := NewRefreshToken(uid, scope, role)

	want := RefreshToken{
		UserID:  uid,
		Scope:   scope,
		Role:    role,
		Expired: Expired(before.Add(12 * time.Hour)),
	}

	opts := cmp.Options{
		expiredAsTime,
		cmpopts.EquateApproxTime(time.Second),
	}
	if diff := cmp.Diff(want, got, opts); diff != "" {
		t.Errorf("NewRefreshToken() mismatch (-want +got):\n%s", diff)
	}
}

// Pins the relative ordering of access and refresh expiries — a regression
// here would suggest createToken's duration argument was wired backwards.
func TestNewAccessToken_expiresBefore_NewRefreshToken(t *testing.T) {
	access := NewAccessToken("u", "s", "r")
	refresh := NewRefreshToken("u", "s", "r")

	accessExp := time.Time(access.Expired)
	refreshExp := time.Time(refresh.Expired)

	if !accessExp.Before(refreshExp) {
		t.Errorf("access expiry %v should be before refresh expiry %v", accessExp, refreshExp)
	}
}

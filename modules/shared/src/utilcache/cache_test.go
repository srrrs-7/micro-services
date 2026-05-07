package utilcache

import (
	"testing"
	"time"
)

func TestCache_makeKey_prefixesWithDash(t *testing.T) {
	cases := []struct {
		name   string
		prefix string
		key    string
		want   string
	}{
		{"plain", "session", "user-1", "session-user-1"},
		{"empty prefix", "", "k", "-k"},
		{"empty key", "session", "", "session-"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewCache(nil, tc.prefix, time.Minute)
			if got := c.makeKey(tc.key); got != tc.want {
				t.Errorf("makeKey(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

// Set/Get/Delete need a real Redis and belong to integration tests
// guarded by a build tag. See .claude/rules/testing.md §5.

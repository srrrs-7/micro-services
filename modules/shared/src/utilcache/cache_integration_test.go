//go:build integration

// Run with: go test -tags=integration ./utilcache/...
// Requires the auth_cache Redis container to be reachable at $REDIS_ADDR
// (default localhost:6379) — see compose.yml.

package utilcache

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultRedisAddr = "localhost:6379"

func redisAddr() string {
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		return v
	}
	return defaultRedisAddr
}

func newTestClient(t *testing.T) *redis.Client {
	t.Helper()
	c, err := NewClient(redisAddr(), "")
	if err != nil {
		t.Skipf("Redis not reachable at %s: %v", redisAddr(), err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// uniquePrefix isolates each subtest's keys so they can run in parallel
// and survive a previous failed run that left keys behind.
func uniquePrefix(t *testing.T) string {
	t.Helper()
	return "test-" + t.Name() + "-" + time.Now().Format("150405.000000")
}

func TestNewClient_pingsAndReturnsError(t *testing.T) {
	if _, err := NewClient("127.0.0.1:1", ""); err == nil {
		t.Fatal("expected dial error for bogus address, got nil")
	}
}

func TestCache_SetGet_roundTripsString(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	c := NewCache(client, uniquePrefix(t), time.Minute)

	if err := c.Set(context.Background(), "key", "hello"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	var got string
	if err := c.Get(context.Background(), "key", &got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "hello" {
		t.Errorf("got = %q, want %q", got, "hello")
	}
}

func TestCache_SetGet_roundTripsInt(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	c := NewCache(client, uniquePrefix(t), time.Minute)

	if err := c.Set(context.Background(), "n", 42); err != nil {
		t.Fatalf("Set: %v", err)
	}

	var got int
	if err := c.Get(context.Background(), "n", &got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != 42 {
		t.Errorf("got = %d, want 42", got)
	}
}

func TestCache_Get_returnsRedisNilForMissingKey(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	c := NewCache(client, uniquePrefix(t), time.Minute)

	var got string
	err := c.Get(context.Background(), "absent", &got)
	if !errors.Is(err, redis.Nil) {
		t.Errorf("err = %v, want redis.Nil", err)
	}
}

func TestCache_Delete_removesKey(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	c := NewCache(client, uniquePrefix(t), time.Minute)
	ctx := context.Background()

	if err := c.Set(ctx, "k", "v"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := c.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var got string
	err := c.Get(ctx, "k", &got)
	if !errors.Is(err, redis.Nil) {
		t.Errorf("after Delete, Get err = %v, want redis.Nil", err)
	}
}

// Delete on a nonexistent key returns nil — Redis DEL returns the number
// of keys deleted, not an error. Pin this behavior so callers can use
// Delete idempotently without an existence check.
func TestCache_Delete_isIdempotent(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	c := NewCache(client, uniquePrefix(t), time.Minute)

	if err := c.Delete(context.Background(), "never-set"); err != nil {
		t.Errorf("Delete on missing key returned %v, want nil", err)
	}
}

func TestCache_TTL_keyExpiresAfterTTL(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	// 1s TTL keeps the suite fast; a 200ms slack covers Redis's second-level
	// TTL precision and the network round trip.
	c := NewCache(client, uniquePrefix(t), time.Second)
	ctx := context.Background()

	if err := c.Set(ctx, "k", "v"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)

	var got string
	err := c.Get(ctx, "k", &got)
	if !errors.Is(err, redis.Nil) {
		t.Errorf("after TTL, Get err = %v, want redis.Nil", err)
	}
}

// Two Cache instances with different prefixes share a client but isolate
// their key spaces — this is the whole reason the prefix exists.
func TestCache_prefixIsolatesKeys(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	prefixBase := uniquePrefix(t)
	a := NewCache(client, prefixBase+"-a", time.Minute)
	b := NewCache(client, prefixBase+"-b", time.Minute)
	ctx := context.Background()

	if err := a.Set(ctx, "k", "from-a"); err != nil {
		t.Fatalf("a.Set: %v", err)
	}
	if err := b.Set(ctx, "k", "from-b"); err != nil {
		t.Fatalf("b.Set: %v", err)
	}

	var gotA, gotB string
	if err := a.Get(ctx, "k", &gotA); err != nil {
		t.Fatalf("a.Get: %v", err)
	}
	if err := b.Get(ctx, "k", &gotB); err != nil {
		t.Fatalf("b.Get: %v", err)
	}
	if gotA != "from-a" {
		t.Errorf("a.Get(k) = %q, want from-a", gotA)
	}
	if gotB != "from-b" {
		t.Errorf("b.Get(k) = %q, want from-b", gotB)
	}
}

// Set replaces an existing value — Redis SET overwrites by default.
func TestCache_Set_overwritesExistingValue(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	c := NewCache(client, uniquePrefix(t), time.Minute)
	ctx := context.Background()

	if err := c.Set(ctx, "k", "v1"); err != nil {
		t.Fatalf("Set v1: %v", err)
	}
	if err := c.Set(ctx, "k", "v2"); err != nil {
		t.Fatalf("Set v2: %v", err)
	}

	var got string
	if err := c.Get(ctx, "k", &got); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "v2" {
		t.Errorf("got = %q, want v2 (overwritten)", got)
	}
}

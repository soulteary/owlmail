package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/soulteary/owlmail/internal/mailbox"
)

func TestClaimCacheBoundsAndRecency(t *testing.T) {
	var cache claimCache
	if _, ok := cache.Load("absent"); ok {
		t.Fatal("zero-value cache returned a claim")
	}
	for i := range maxCachedClaims {
		cache.Store(fmt.Sprint(i), "false:")
	}
	cache.Load("0") // The first inserted entry is now the most recently used.
	cache.Store("overflow", "true:alice@example.test")
	if _, ok := cache.Load("1"); ok {
		t.Fatal("least recently used entry was not evicted")
	}
	if _, ok := cache.Load("0"); !ok {
		t.Fatal("recently used entry was evicted")
	}
	cache.Store("0", "true:changed@example.test")
	if evidence, ok := cache.Load("0"); !ok || evidence != "true:changed@example.test" {
		t.Fatal("evidence update was lost")
	}
	if len(cache.entries) != maxCachedClaims || cache.recent.Len() != maxCachedClaims {
		t.Fatal("cache exceeded its entry bound")
	}
}

func TestClaimCacheConcurrentEviction(t *testing.T) {
	var cache claimCache
	var workers sync.WaitGroup
	for worker := range 16 {
		workers.Go(func() {
			for i := range 1000 {
				key := fmt.Sprintf("%d:%d", worker, i)
				cache.Store(key, key)
				if evidence, ok := cache.Load(key); ok && evidence != key {
					t.Errorf("different subject's evidence returned for %s", key)
				}
			}
		})
	}
	workers.Wait()
	if len(cache.entries) != maxCachedClaims || cache.recent.Len() != maxCachedClaims {
		t.Fatal("concurrent writes exceeded the cache bound")
	}
}

func TestMultiuserClaimCacheEvictionPreservesAuthority(t *testing.T) {
	f := newMultiFixture(t)
	f.mail(t, "alice", "alice-id")
	f.mail(t, "bob", "bob-id")
	if response := f.request(t, "alice", "GET", "/api/v1/emails", ""); response.StatusCode != http.StatusOK {
		t.Fatal("initial authenticated lookup failed")
	}
	for i := range maxCachedClaims {
		f.api.multiuserClaims.Store(fmt.Sprintf("unrelated:%d", i), "false:")
	}
	if _, ok := f.api.multiuserClaims.Load("main\x00alice"); ok {
		t.Fatal("test did not evict the authenticated claim")
	}
	response := f.request(t, "alice", "GET", "/api/v1/emails/bob-id", "")
	if response.StatusCode != http.StatusNotFound {
		t.Fatal("eviction bypassed mailbox isolation")
	}
	box, err := f.store.PersonalMailbox(context.Background(), "main", "alice")
	if err != nil || box != f.boxes["alice"] {
		t.Fatal("eviction changed durable ownership")
	}
	// A warmed cache is never a replacement for verifying the incoming headers.
	request := httptest.NewRequest("GET", "http://localhost/api/v1/emails", nil)
	request.Header.Set("X-Auth-User", "alice")
	request.Header.Set("X-Auth-Email-Verified", "invalid")
	response, err = f.api.app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatal("cached claim bypassed invalid upstream evidence")
	}
}

func TestMultiuserClaimCacheRebindsChangedEvidence(t *testing.T) {
	f := newMultiFixture(t)
	if response := f.request(t, "alice", "GET", "/api/v1/emails", ""); response.StatusCode != http.StatusOK {
		t.Fatal("initial authenticated lookup failed")
	}
	request := httptest.NewRequest("GET", "http://localhost/api/v1/emails", nil)
	request.Header.Set("X-Auth-User", "alice")
	request.Header.Set("X-Auth-Email", "alice@example.test")
	request.Header.Set("X-Auth-Email-Verified", "true")
	response, err := f.api.app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatal("changed evidence was not accepted")
	}
	// A durable bound route proves that changing the cached evidence did not
	// suppress BindLogin. EnsurePending returns the existing route unchanged.
	route, err := f.store.EnsurePending(context.Background(), "main", "alice@example.test", 10)
	if err != nil || route.State != "bound" || route.MailboxID != f.boxes["alice"] {
		t.Fatalf("changed verified evidence did not bind the route: %+v %v", route, err)
	}
}

func BenchmarkClaimCacheBinding(b *testing.B) {
	store, err := mailbox.Open(filepath.Join(b.TempDir(), "control.sqlite"), true)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = store.Close() })
	login := mailbox.Login{Namespace: "main", Subject: "alice"}
	if _, err := store.BindLogin(context.Background(), login, mailbox.ExplicitHistory); err != nil {
		b.Fatal(err)
	}
	b.Run("cached_with_authority_lookup", func(b *testing.B) {
		var cache claimCache
		cache.Store("main\x00alice", "false:")
		b.ReportAllocs()
		for b.Loop() {
			if _, ok := cache.Load("main\x00alice"); !ok {
				b.Fatal("cache miss")
			}
			if _, err := store.PersonalMailbox(context.Background(), "main", "alice"); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("repeated_binding", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := store.PersonalMailbox(context.Background(), "main", "alice"); err != nil {
				b.Fatal(err)
			}
			if _, err := store.BindLogin(context.Background(), login, mailbox.ExplicitHistory); err != nil {
				b.Fatal(err)
			}
		}
	})
}

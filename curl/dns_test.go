package curl

import (
	"context"
	"testing"
	"time"
)

func TestCachedLookup(t *testing.T) {
	// Miss resolves via real DNS and caches.
	ip, err := cachedLookup(context.Background(), "github.com")
	if err != nil {
		t.Skipf("no network: %v", err)
	}
	v, ok := dnsCache.Load("github.com")
	if !ok {
		t.Fatal("cachedLookup did not store the answer")
	}
	e := v.(dnsCacheEntry)
	if !e.ip.Equal(ip) {
		t.Errorf("cache holds %v, lookup returned %v", e.ip, ip)
	}

	// Hit returns the cached answer without a new query (negative TTL: an
	// expired entry must be dropped, not served). The re-resolved IP may
	// legitimately differ (CDN round-robin) — only cache mechanics matter.
	dnsCache.Store("github.com", dnsCacheEntry{ip: ip, expiry: time.Now().Add(-time.Second)})
	got, err := cachedLookup(context.Background(), "github.com")
	if err != nil {
		t.Skipf("no network: %v", err)
	}
	if got == nil {
		t.Error("re-lookup after expiry returned nil")
	}
	if v, ok := dnsCache.Load("github.com"); !ok || v.(dnsCacheEntry).expiry.Before(time.Now()) {
		t.Error("expired entry not refreshed")
	}
}

func TestHostPort(t *testing.T) {
	if h, p := hostPort("https://example.com/x"); h != "example.com" || p != "443" {
		t.Errorf("hostPort(https) = %q, %q", h, p)
	}
	if h, p := hostPort("http://example.com:8080/x"); h != "example.com" || p != "8080" {
		t.Errorf("hostPort(port) = %q, %q", h, p)
	}
}

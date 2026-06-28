package main

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestNextBackendRoundRobin(t *testing.T) {
	ps := &proxyServer{
		routes: routes{
			Route: map[string][]string{
				"/user": {"a", "b", "c"},
			},
		},
	}

	first, ok := ps.nextBackend("/user")
	if !ok || first != "a" {
		t.Fatalf("expected first backend a, got %q, ok=%v", first, ok)
	}

	second, ok := ps.nextBackend("/user")
	if !ok || second != "b" {
		t.Fatalf("expected second backend b, got %q, ok=%v", second, ok)
	}

	third, ok := ps.nextBackend("/user")
	if !ok || third != "c" {
		t.Fatalf("expected third backend c, got %q, ok=%v", third, ok)
	}
}

func TestVerifyToken(t *testing.T) {
	example := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.vgmErpVc7vMC8DXeL8s0WiVH8lg8Tw-78t6QzIEu4WU"
	if err := verify(example); err != nil {
		t.Fatalf("verify returned error: %v", err)
	}
}

func TestAuthenticateRejectsMalformedHeader(t *testing.T) {
	r, err := http.NewRequest(http.MethodGet, "/user", bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	if err := authenticate(r); err == nil {
		t.Fatal("expected authenticate to fail without Authorization header")
	}

	r.Header.Set("Authorization", "Token abc")
	if err := authenticate(r); err == nil {
		t.Fatal("expected authenticate to reject non-Bearer header")
	}
}

func TestResponseCache(t *testing.T) {
	cache := newResponseCache(25*time.Millisecond, 2)
	cache.set("k1", cachedResponse{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       []byte("hello"),
		ExpiresAt:  time.Now().Add(25 * time.Millisecond),
	})

	entry, ok := cache.get("k1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if entry.StatusCode != http.StatusOK || string(entry.Body) != "hello" {
		t.Fatalf("unexpected cache entry: %+v", entry)
	}

	time.Sleep(30 * time.Millisecond)
	if _, ok := cache.get("k1"); ok {
		t.Fatal("expected cache entry to expire")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := loadConfig("config.yaml")
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}

	if cfg.Server.HTTPAddress == "" || cfg.Server.HTTPSAddress == "" {
		t.Fatal("expected server defaults to be populated")
	}
	if cfg.Cache.TTLSeconds <= 0 || cfg.Cache.MaxEntries <= 0 {
		t.Fatal("expected cache defaults to be populated")
	}
}

func TestRouteKeyFromURL(t *testing.T) {
	if got := routeKeyFromURL("/user?x=1"); got != "/user" {
		t.Fatalf("expected /user, got %q", got)
	}
}

func TestYAMLCompatibility(t *testing.T) {
	var cfg appConfig
	if err := yaml.Unmarshal([]byte("routes:\n  /x:\n    - http://localhost:1\n"), &cfg); err != nil {
		t.Fatalf("yaml unmarshal failed: %v", err)
	}
	if len(cfg.Routes["/x"]) != 1 {
		t.Fatal("expected route to unmarshal")
	}
}

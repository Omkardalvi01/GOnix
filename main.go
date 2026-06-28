package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gopkg.in/yaml.v3"
)

type routes struct {
	Route map[string][]string `yaml:"routes"`
}

type tlsSettings struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type serverSettings struct {
	HTTPAddress  string      `yaml:"http_address"`
	HTTPSAddress string      `yaml:"https_address"`
	RedirectHTTP bool        `yaml:"redirect_http"`
	TLS          tlsSettings `yaml:"tls"`
}

type cacheSettings struct {
	Enabled    bool `yaml:"enabled"`
	TTLSeconds int  `yaml:"ttl_seconds"`
	MaxEntries int  `yaml:"max_entries"`
}

type appConfig struct {
	Routes map[string][]string `yaml:"routes"`
	Server serverSettings      `yaml:"server"`
	Cache  cacheSettings       `yaml:"cache"`
}

type cachedResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	ExpiresAt  time.Time
}

type responseCache struct {
	mu         sync.Mutex
	ttl        time.Duration
	maxEntries int
	items      map[string]cachedResponse
	order      []string
}

type proxyServer struct {
	routes  routes
	routeMu sync.Mutex
	client  *http.Client
	cache   *responseCache
}

var secretKey = []byte("stupidstupidstupidstupidstupidstupid")
var INVALID_TOKEN = errors.New("Invalid Token")
var config_file = "config.yaml"

func defaultAppConfig() appConfig {
	return appConfig{
		Server: serverSettings{
			HTTPAddress:  ":5000",
			HTTPSAddress: ":5443",
		},
		Cache: cacheSettings{
			Enabled:    true,
			TTLSeconds: 30,
			MaxEntries: 1024,
		},
	}
}

func loadConfig(path string) (appConfig, error) {
	cfg := defaultAppConfig()

	f, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	if err := yaml.Unmarshal(f, &cfg); err != nil {
		return cfg, err
	}

	if cfg.Server.HTTPAddress == "" {
		cfg.Server.HTTPAddress = ":5000"
	}
	if cfg.Server.HTTPSAddress == "" {
		cfg.Server.HTTPSAddress = ":5443"
	}
	if cfg.Cache.TTLSeconds <= 0 {
		cfg.Cache.TTLSeconds = 30
	}
	if cfg.Cache.MaxEntries <= 0 {
		cfg.Cache.MaxEntries = 1024
	}

	return cfg, nil
}

func newProxyServer(r routes, cfg appConfig) *proxyServer {
	transport := &http.Transport{
		IdleConnTimeout:     10 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     10,
	}

	var cache *responseCache
	if cfg.Cache.Enabled {
		cache = newResponseCache(time.Duration(cfg.Cache.TTLSeconds)*time.Second, cfg.Cache.MaxEntries)
	}

	return &proxyServer{
		routes: r,
		client: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		cache: cache,
	}
}

func newResponseCache(ttl time.Duration, maxEntries int) *responseCache {
	return &responseCache{
		ttl:        ttl,
		maxEntries: maxEntries,
		items:      make(map[string]cachedResponse),
	}
}

func (c *responseCache) enabled() bool {
	return c != nil && c.ttl > 0 && c.maxEntries > 0
}

func (c *responseCache) get(key string) (cachedResponse, bool) {
	if c == nil {
		return cachedResponse{}, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[key]
	if !ok {
		return cachedResponse{}, false
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(c.items, key)
		return cachedResponse{}, false
	}

	return cachedResponse{
		StatusCode: entry.StatusCode,
		Header:     cloneHeader(entry.Header),
		Body:       append([]byte(nil), entry.Body...),
		ExpiresAt:  entry.ExpiresAt,
	}, true
}

func (c *responseCache) set(key string, entry cachedResponse) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.items[key]; !exists {
		c.order = append(c.order, key)
	}

	c.items[key] = entry
	c.evictLocked()
}

func (c *responseCache) evictLocked() {
	if c == nil {
		return
	}

	for len(c.items) > c.maxEntries && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		if _, ok := c.items[oldest]; ok {
			delete(c.items, oldest)
		}
	}
}

func routeKeyFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err == nil && parsed.Path != "" {
		return parsed.Path
	}

	if i := strings.Index(raw, "?"); i >= 0 {
		return raw[:i]
	}

	return raw
}

func get_path(u string, r routes) string {
	path := routeKeyFromURL(u)
	backends := r.Route[path]
	if len(backends) == 0 {
		return ""
	}

	selected := backends[0]
	r.Route[path] = append(backends[1:], selected)
	return selected
}

func verify(tok string) error {
	token, err := jwt.Parse(tok, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %s", t.Method.Alg())
		}
		return secretKey, nil
	})
	if err != nil {
		return err
	}
	if !token.Valid {
		return INVALID_TOKEN
	}
	return nil
}

func authenticate(r *http.Request) error {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "" {
		return errors.New("missing authorization header")
	}

	fields := strings.Fields(authHeader)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return errors.New("invalid authorization header")
	}

	return verify(fields[1])
}

func (p *proxyServer) nextBackend(path string) (string, bool) {
	p.routeMu.Lock()
	defer p.routeMu.Unlock()

	backends, ok := p.routes.Route[path]
	if !ok || len(backends) == 0 {
		return "", false
	}

	selected := backends[0]
	p.routes.Route[path] = append(backends[1:], selected)
	return selected, true
}

func (p *proxyServer) forward(r *http.Request, backend string) (*http.Response, error) {
	target, err := url.Parse(backend)
	if err != nil {
		return nil, err
	}
	target.RawQuery = r.URL.RawQuery

	req, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), r.Body)
	if err != nil {
		return nil, err
	}

	req.Header = cloneHeader(r.Header)
	req.Host = target.Host
	req.ContentLength = r.ContentLength
	req.Header.Set("X-Forwarded-For", clientIP(r))
	if r.TLS != nil {
		req.Header.Set("X-Forwarded-Proto", "https")
	} else {
		req.Header.Set("X-Forwarded-Proto", "http")
	}

	return p.client.Do(req)
}

func (p *proxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := authenticate(r); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, "Unauthorized: %v", err)
		return
	}

	backend, ok := p.nextBackend(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	logging(r, backend)

	if p.cache != nil && p.cache.enabled() && r.Method == http.MethodGet {
		key := cacheKey(r, backend)
		if entry, ok := p.cache.get(key); ok {
			writeCachedResponse(w, entry)
			return
		}

		resp, err := p.forward(r, backend)
		if err != nil {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}

		if shouldCacheResponse(resp) {
			p.cache.set(key, cachedResponse{
				StatusCode: resp.StatusCode,
				Header:     cloneHeader(resp.Header),
				Body:       append([]byte(nil), body...),
				ExpiresAt:  time.Now().Add(p.cache.ttl),
			})
		}

		writeResponse(w, resp.StatusCode, resp.Header, body)
		return
	}

	resp, err := p.forward(r, backend)
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	writeStreamResponse(w, resp)
}

func cacheKey(r *http.Request, backend string) string {
	auth := r.Header.Get("Authorization")
	sum := sha256.Sum256([]byte(auth))
	return strings.Join([]string{
		r.Method,
		backend,
		r.URL.Path,
		r.URL.RawQuery,
		hex.EncodeToString(sum[:]),
	}, "|")
}

func shouldCacheResponse(resp *http.Response) bool {
	if resp == nil || resp.StatusCode != http.StatusOK {
		return false
	}
	if resp.Header.Get("Set-Cookie") != "" {
		return false
	}
	cacheControl := strings.ToLower(resp.Header.Get("Cache-Control"))
	if strings.Contains(cacheControl, "no-store") {
		return false
	}
	return true
}

func writeCachedResponse(w http.ResponseWriter, entry cachedResponse) {
	copyHeaders(w.Header(), entry.Header)
	w.WriteHeader(entry.StatusCode)
	_, _ = w.Write(entry.Body)
}

func writeResponse(w http.ResponseWriter, statusCode int, headers http.Header, body []byte) {
	copyHeaders(w.Header(), headers)
	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
}

func writeStreamResponse(w http.ResponseWriter, resp *http.Response) {
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func copyHeaders(dest, src http.Header) {
	for k, values := range src {
		if isHopByHopHeader(k) {
			continue
		}
		dest.Del(k)
		for _, v := range values {
			dest.Add(k, v)
		}
	}
}

func cloneHeader(src http.Header) http.Header {
	dst := make(http.Header, len(src))
	for k, values := range src {
		dst[k] = append([]string(nil), values...)
	}
	return dst
}

func isHopByHopHeader(key string) bool {
	switch http.CanonicalHeaderKey(key) {
	case "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade":
		return true
	default:
		return false
	}
}

func clientIP(r *http.Request) string {
	if prior := r.Header.Get("X-Forwarded-For"); prior != "" {
		return prior + ", " + hostWithoutPort(r.RemoteAddr)
	}
	return hostWithoutPort(r.RemoteAddr)
}

func hostWithoutPort(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func logging(r *http.Request, b string) {
	fmt.Printf("time : %v , url : %v , forwarded_url : %v\n", time.Now(), r.URL, b)
}

func redirectToHTTPS(httpsAddress string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := *r.URL
		target.Scheme = "https"
		target.Host = redirectHost(r.Host, httpsAddress)
		http.Redirect(w, r, target.String(), http.StatusMovedPermanently)
	})
}

func redirectHost(requestHost, httpsAddress string) string {
	port := httpsPort(httpsAddress)
	if requestHost == "" {
		return requestHost
	}

	host, _, err := net.SplitHostPort(requestHost)
	if err != nil {
		if port == "" {
			return requestHost
		}
		return net.JoinHostPort(requestHost, port)
	}

	if port == "" {
		return requestHost
	}

	return net.JoinHostPort(host, port)
}

func httpsPort(addr string) string {
	if addr == "" {
		return ""
	}

	_, port, err := net.SplitHostPort(addr)
	if err == nil {
		return port
	}

	if strings.HasPrefix(addr, ":") {
		return strings.TrimPrefix(addr, ":")
	}

	return ""
}

func run(cfg appConfig, handler http.Handler) {
	if cfg.Server.TLS.Enabled {
		if cfg.Server.RedirectHTTP {
			go func() {
				log.Printf("starting HTTP redirect server on %s", cfg.Server.HTTPAddress)
				if err := http.ListenAndServe(cfg.Server.HTTPAddress, redirectToHTTPS(cfg.Server.HTTPSAddress)); err != nil {
					log.Fatal(err)
				}
			}()
		}

		log.Printf("starting HTTPS server on %s", cfg.Server.HTTPSAddress)
		log.Fatal(http.ListenAndServeTLS(cfg.Server.HTTPSAddress, cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile, handler))
		return
	}

	log.Printf("starting HTTP server on %s", cfg.Server.HTTPAddress)
	log.Fatal(http.ListenAndServe(cfg.Server.HTTPAddress, handler))
}

func main() {
	cfg, err := loadConfig(config_file)
	if err != nil {
		fmt.Println(err)
	}

	proxy := newProxyServer(routes{Route: cfg.Routes}, cfg)
	run(cfg, proxy)
}

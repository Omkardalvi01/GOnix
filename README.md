# Go Reverse Proxy with JWT Authentication, SSL Termination, and Caching

This is a small reverse proxy written in Go. It supports:

* JWT-based authentication
* Round-robin backend selection
* Request logging
* YAML-based route configuration
* Optional HTTPS termination at the proxy layer
* In-memory caching for cacheable `GET` responses

## Features

* Reverse proxying to backend servers.
* JWT auth before forwarding requests.
* Round-robin routing per configured path.
* Optional TLS listener so the proxy can terminate SSL/TLS and forward plain HTTP to backends.
* Optional HTTP-to-HTTPS redirect.
* In-memory response cache with TTL and max-entry limits.

## Configuration

`config.yaml` supports three top-level sections:

```yaml
routes:
  /user:
    - http://localhost:4000/user/v1
    - http://localhost:4000/user/v2
  /admin:
    - http://localhost:4000/admin/v1

server:
  http_address: ":5000"
  https_address: ":5443"
  redirect_http: false
  tls:
    enabled: false
    cert_file: "certs/server.crt"
    key_file: "certs/server.key"

cache:
  enabled: true
  ttl_seconds: 30
  max_entries: 1024
```

## SSL Termination

When `server.tls.enabled` is `true`, the proxy listens on `server.https_address` using the certificate and key you provide. Backends still receive plain HTTP requests from the proxy.

If `server.redirect_http` is also `true`, the proxy starts a second HTTP listener on `server.http_address` and redirects traffic to HTTPS.

Example self-signed cert generation:

```bash
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout certs/server.key \
  -out certs/server.crt \
  -days 365 \
  -subj "/CN=localhost"
```

## Caching

Caching is in-memory and only applies to successful `GET` responses.

Cache keys include:

* HTTP method
* Selected backend
* Request path and query string
* Authorization header fingerprint

That keeps cached responses scoped to the same backend route and token.

## Setup

### 1. Run the proxy

```bash
go run .
```

By default, it listens on `:5000` with HTTP only.

### 2. Enable TLS

Set the following in `config.yaml`:

```yaml
server:
  tls:
    enabled: true
    cert_file: "certs/server.crt"
    key_file: "certs/server.key"
  redirect_http: true
```

## Authentication

Send a bearer token in the `Authorization` header:

```http
Authorization: Bearer <your_jwt_token>
```

The token is verified with `HS256` using the shared secret in `main.go`.

## Example Request

```bash
curl -k https://localhost:5443/user \
  -H "Authorization: Bearer <your_jwt_token>"
```

Use `-k` only with self-signed certificates.

## Testing

```bash
GOCACHE=/tmp/gocache go test ./...
```

## Notes

* The proxy still forwards to HTTP backends; TLS is terminated at the proxy.
* Route rotation is now protected by a mutex.
* The cache is process-local and resets when the proxy restarts.

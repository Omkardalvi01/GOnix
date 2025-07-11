# 🔁 Go Reverse Proxy with JWT Authentication & Load Balancing

This project is a simple reverse proxy server written in Go. It supports:

* ✅ **JWT-based authentication**
* ✅ **Round-robin load balancing**
* ✅ **Basic request logging**
* ✅ **Dynamic route configuration via YAML**

---

## 📦 Features

* **Reverse Proxy:** Forwards incoming HTTP requests to backend servers.
* **JWT Authentication:** Ensures only authorized requests are forwarded.
* **Round-Robin Routing:** Distributes requests across multiple backends for a given route.
* **Request Logging:** Logs timestamp, original URL, and forwarded URL.
* **Configurable Routes:** Routes are defined in a `config.yaml` file.

---

## 💠 Setup

### 1. Clone the repository

```bash
git clone https://github.com/your-username/go-reverse-proxy.git
cd go-reverse-proxy
```

### 2. Create a `config.yaml` file

Example:

```yaml
routes:
  /api/service1:
    - http://localhost:8001
    - http://localhost:8002
  /api/service2:
    - http://localhost:9001
```

### 3. Run the server

```bash
go run main.go
```

Server runs on port `5000`.

---

## 🔐 Authentication

* Include a **JWT** in the `Authorization` header as a **Bearer token**:

```http
Authorization: Bearer <your_token_here>
```

* Tokens are verified using a symmetric key (`HS256`).
* You can modify the secret key in the `secretKey` variable inside `main.go`.

---

## 🧪 Example Request

```bash
curl -X GET http://localhost:5000/api/service1 \
  -H "Authorization: Bearer <your_jwt_token>"
```

---

## 🤩 Dependencies

* [`github.com/golang-jwt/jwt/v5`](https://pkg.go.dev/github.com/golang-jwt/jwt/v5) — For JWT parsing and validation.
* [`gopkg.in/yaml.v3`](https://pkg.go.dev/gopkg.in/yaml.v3) — For reading route configs from YAML.

Install dependencies:

```bash
go mod init reverse-proxy
go get github.com/golang-jwt/jwt/v5
go get gopkg.in/yaml.v3
```

---

## 📌 Notes

* This proxy supports **only HTTP** (not HTTPS).
* The current route rotation logic is not concurrency-safe.
* For production-grade systems, consider mutexes or concurrent-safe data structures.

---

## 📄 License

MIT License

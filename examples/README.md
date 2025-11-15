# Examples

This directory contains example usage of the distance-hashing library.

## Running Examples

All examples are standalone Go programs. Run them with:

```bash
go run examples/<filename>.go
```

## Available Examples

### 1. basic_usage.go

Demonstrates basic session unification with `SessionGenerator` (N-Degree Hash).

```bash
go run examples/basic_usage.go
```

**Shows:**
- Creating a session generator
- Anonymous user session tracking
- Linking identifiers on login
- JWT token issuance
- Session key stability verification

### 2. canonical_usage.go

Demonstrates priority-based session keys with `CanonicalSessionGenerator` (recommended for production).

```bash
go run examples/canonical_usage.go
```

**Shows:**
- Priority-based canonical identifier selection
- Session key evolution as identifiers are added
- Key stability when canonical identifier is established
- Priority order: UserID > Email > ClientID > DeviceID > CookieID > JwtToken

### 3. unionfind_usage.go

Demonstrates raw `UnionFind` data structure for connectivity checks.

```bash
go run examples/unionfind_usage.go
```

**Shows:**
- Basic Union-Find operations
- Connectivity verification
- Component size calculation
- Fraud detection use case (shared device detection)

### 4. http_middleware.go

Demonstrates HTTP middleware integration for session tracking in web applications.

```bash
go run examples/http_middleware.go
```

Then test with:

```bash
# Get session key
curl -H 'Cookie: session_id=abc123' http://localhost:8080/

# Login (links cookie to user)
curl -H 'Cookie: session_id=abc123' -X POST -d 'user_id=user_42' http://localhost:8080/login

# Get statistics
curl http://localhost:8080/stats
```

**Shows:**
- HTTP middleware pattern
- Extracting identifiers from headers, cookies, and JWT
- Linking identifiers on login
- Exposing session statistics

## Use Cases Demonstrated

1. **Session Tracking**: Track anonymous → authenticated user journey
2. **Cross-Device**: Unify user across multiple devices
3. **Fraud Detection**: Identify related accounts through shared identifiers
4. **HTTP Integration**: Production-ready middleware pattern

## Next Steps

- Integrate with your authentication system
- Add Redis persistence for failover recovery
- Stream session events to Kafka/ClickHouse
- Implement session expiration policies

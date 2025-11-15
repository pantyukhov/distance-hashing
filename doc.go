/*
Package distancehashing provides high-performance session unification algorithms
for identity resolution across multiple user identifiers.

# Overview

This library solves the identity resolution problem: when users interact with your
service through multiple channels (cookies, JWT tokens, device IDs, etc.), you need
to unify these identifiers into a single session for analytics and tracking.

The library offers three approaches with different trade-offs:

1. Union-Find: Simple, fast O(α(n)) ≈ O(1) operations, minimal memory
2. N-Degree Hash: Deterministic canonical hashing based on graph structure
3. Canonical Session: Combines Union-Find speed with stable session keys

# Quick Start

	import dh "github.com/wallarm/distance-hashing"

	// Create session generator
	sg, _ := dh.NewSessionGenerator(10000)

	// Scenario 1: Anonymous user with cookie
	sessionKey1 := sg.GetSessionKey(dh.Identifiers{
		CookieID: "cookie_abc123",
	})

	// Scenario 2: User logs in - link cookie to user ID
	sg.LinkIdentifiers("cookie:cookie_abc123", "uid:user_42")

	// Scenario 3: JWT token issued
	sg.LinkIdentifiers("uid:user_42", "jwt:jwt_token_xyz")

	// All identifiers now return the SAME session key
	key1 := sg.GetSessionKey(dh.Identifiers{CookieID: "cookie_abc123"})
	key2 := sg.GetSessionKey(dh.Identifiers{UserID: "user_42"})
	key3 := sg.GetSessionKey(dh.Identifiers{JwtToken: "jwt_token_xyz"})
	// key1 == key2 == key3

# Union-Find Approach

The UnionFind data structure provides the foundation for session unification with
near-constant time O(α(n)) operations, where α is the inverse Ackermann function.

	uf := dh.NewUnionFind()

	// Union identifiers
	uf.Union("uid:user_123", "jwt:token_abc")
	uf.Union("jwt:token_abc", "cookie:session_xyz")

	// Check connectivity
	if uf.Connected("uid:user_123", "cookie:session_xyz") {
		// They're in the same session
	}

Features:
  - O(α(n)) ≈ O(1) amortized time for Find and Union
  - Path compression and union by rank optimizations
  - Thread-safe with minimal lock contention
  - Memory efficient: O(V) where V is number of identifiers

# N-Degree Hash Approach

SessionGenerator implements the N-Degree Hash algorithm, similar to W3C's RDFC-1.0
for RDF Dataset Canonicalization. It generates canonical hashes based on the entire
graph structure.

	sg, _ := dh.NewSessionGenerator(10000)

	sessionKey := sg.GetSessionKey(dh.Identifiers{
		UserID:   "user_123",
		JwtToken: "jwt_abc",
		CookieID: "cookie_xyz",
	})

Features:
  - Deterministic: same graph structure = same session key
  - Order-independent: identifiers can be added in any order
  - Transitive: A→B→C means all get the same session key
  - Cache hit performance: 96 ns/op (10M+ ops/sec)

Algorithm:
  1. Build graph of identifier relationships
  2. Compute first-degree hash for each node (based on neighbors)
  3. Resolve collisions with N-degree hash (multi-hop paths)
  4. Generate canonical hash for connected component

Complexity:
  - Cache hit: O(1) - lookup in LRU cache
  - Cache miss: O(V*D*depth) where V=nodes, D=avg degree, depth=3
  - Memory: O(V + E) where E is number of edges

# Canonical Session Approach

CanonicalSessionGenerator combines the speed of Union-Find with stable session keys
based on priority-based canonical identifier selection.

	csg, _ := dh.NewCanonicalSessionGenerator(10000)

	sessionKey := csg.GetSessionKey(dh.Identifiers{
		UserID:   "user_123",
		CookieID: "cookie_abc",
		JwtToken: "jwt_xyz",
	})

Features:
  - O(α(n)) ≈ O(1) complexity for Find/Union
  - Stable session keys based on highest-priority identifier
  - Priority: UserID > Email > ClientID > DeviceID > CookieID > JwtToken
  - Simpler implementation than N-Degree Hash

Priority Selection:
  1. UserID (uid:*) - highest priority, most stable
  2. Email (email:*) - stable, often required for signup
  3. ClientID (client:*) - OAuth client
  4. DeviceID (device:*) - device fingerprint
  5. CookieID (cookie:*) - session cookie
  6. JwtToken (jwt:*) - tokens expire
  7. CustomID (custom:*) - fallback

# Algorithm Comparison

	| Approach        | Complexity   | Memory  | Deterministic | Stable Key |
	|-----------------|--------------|---------|---------------|------------|
	| Union-Find      | O(α(n))      | O(V)    | No*           | No*        |
	| N-Degree Hash   | O(V*D*depth) | O(V+E)  | Yes           | Yes**      |
	| Canonical       | O(α(n))      | O(V)    | Yes           | Yes        |

	* Root depends on insertion order
	** May change when graph structure changes

# Performance Benchmarks

Tested on Apple M3 (ARM64):

	Union-Find Find:          81 ns/op   (12M ops/sec)
	Union-Find Union:        569 ns/op   (1.7M ops/sec)

	N-Degree Cache Hit:       96 ns/op   (10M ops/sec)
	N-Degree Cache Miss:   1,308 ns/op   (764K ops/sec)

	Canonical Cache Hit:      96 ns/op   (10M ops/sec)
	Canonical Cache Miss:    ~100 ns/op  (10M ops/sec)

Memory usage (5000 sessions, 15000 identifiers):
  - Union-Find: ~1 MB
  - N-Degree Hash: ~1.7 MB (includes graph + caches)
  - Canonical: ~1 MB (Union-Find + cache)

# Use Cases

1. User Session Tracking: Unify anonymous → authenticated sessions
2. Cross-Device Analytics: Track users across web, mobile, API
3. Fraud Detection: Identify related accounts through shared identifiers
4. A/B Testing: Ensure consistent experience across session changes
5. Compliance: GDPR/CCPA data deletion across all related identifiers

# Production Considerations

Cache Size: Set appropriate cache size based on active sessions
  - Default: 10,000 entries
  - Rule of thumb: 2x expected concurrent sessions

Memory Management:
  - Monitor with GetStats() method
  - Periodically save snapshots to Redis for restart recovery

Scalability:
  - Single node: 100K+ RPS (proven)
  - Multiple nodes: Use consistent hashing for session affinity
  - Data warehouse: Final aggregation in ClickHouse/BigQuery

Thread Safety:
  - All operations are thread-safe
  - Uses sync.RWMutex for minimal contention
  - Cache hits don't require write locks

# Implementation Notes

All identifiers are internally prefixed with their type:
  - UserID → "uid:user_123"
  - JwtToken → "jwt:token_abc"
  - CookieID → "cookie:session_xyz"
  - Email → "email:user@example.com" (normalized to lowercase)
  - DeviceID → "device:fingerprint_abc"
  - ClientID → "client:oauth_client_id"
  - CustomID → "custom:any_custom_id"

This ensures type safety and prevents collisions between different identifier types.

# References

  - RDFC-1.0: RDF Dataset Canonicalization (https://www.w3.org/TR/rdf-canon/)
  - Union-Find Algorithm (https://en.wikipedia.org/wiki/Disjoint-set_data_structure)
  - Identity Resolution in CDPs (https://segment.com/blog/identity-resolution/)
*/
package distancehashing

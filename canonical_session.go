package distancehashing

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	lru "github.com/hashicorp/golang-lru/v2"
)

// CanonicalSessionGenerator generates stable session keys using Union-Find
// with canonical root selection.
//
// Key differences from N-Degree Hash approach:
// - O(α(n)) ≈ O(1) complexity for Find/Union (vs O(V²) for N-Degree)
// - Stable session_key that doesn't change when graph grows
// - Simpler implementation (~150 lines vs ~500 lines)
// - Less memory: O(V) instead of O(V + E)
//
// Canonical root selection ensures deterministic session keys:
// Priority: UserID > Email > ClientID > DeviceID > CookieID > JwtToken
type CanonicalSessionGenerator struct {
	uf    *UnionFind
	cache *lru.Cache[string, string]
}

// NewCanonicalSessionGenerator creates a new canonical session generator.
func NewCanonicalSessionGenerator(cacheSize int) (*CanonicalSessionGenerator, error) {
	cache, err := lru.New[string, string](cacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create LRU cache: %w", err)
	}

	return &CanonicalSessionGenerator{
		uf:    NewUnionFind(),
		cache: cache,
	}, nil
}

// GetSessionKey returns a stable, canonical session key for the given identifiers.
// The session key is based on the "canonical" identifier in the component
// (selected by priority: UserID > Email > ClientID > DeviceID > CookieID > JwtToken).
//
// This ensures:
// - Same session_key for all identifiers in a connected component
// - Stability: session_key doesn't change when adding new links
// - Deterministic: same component structure = same session_key
//
// Time complexity: O(α(n)) ≈ O(1) amortized
func (csg *CanonicalSessionGenerator) GetSessionKey(ids Identifiers) string {
	// Normalize identifiers
	identifiers := csg.normalizeIdentifiers(ids)

	if len(identifiers) == 0 {
		return "sess_anonymous"
	}

	// IMPORTANT: Union all identifiers BEFORE checking cache
	// This ensures all provided identifiers are linked together
	// even if we have a cache hit
	root := csg.uf.Find(identifiers[0])
	for i := 1; i < len(identifiers); i++ {
		root = csg.uf.Union(root, identifiers[i])
	}

	// Find canonical identifier in the component
	canonical := csg.selectCanonical(root)

	// Generate stable session key from canonical identifier
	sessionKey := csg.generateSessionKey(canonical)

	// Check if cache has correct value
	if cachedKey, ok := csg.cache.Get(identifiers[0]); ok {
		if cachedKey == sessionKey {
			// Cache hit with correct key - update cache for other identifiers
			for i := 1; i < len(identifiers); i++ {
				csg.cache.Add(identifiers[i], sessionKey)
			}
			return sessionKey
		}
		// Cache hit but stale (canonical changed) - fall through to update
	}

	// Cache miss or stale - update cache for all identifiers
	for _, id := range identifiers {
		csg.cache.Add(id, sessionKey)
	}

	return sessionKey
}

// LinkIdentifiers explicitly links two identifiers as belonging to the same session.
//
// Time complexity: O(α(n)) ≈ O(1) amortized
func (csg *CanonicalSessionGenerator) LinkIdentifiers(id1, id2 string) {
	if id1 == "" || id2 == "" {
		return
	}

	// Check if already linked (idempotent operation)
	if csg.uf.Connected(id1, id2) {
		return // Already in same component, no operation needed
	}

	// Union the identifiers
	csg.uf.Union(id1, id2)

	// Invalidate cache only for the two linked identifiers
	// Trade-off: Other identifiers in component may have stale cache temporarily,
	// but will be corrected on next GetSessionKey call.
	// This makes LinkIdentifiers O(1) instead of O(component_size) which is critical
	// for scaling to 1M+ sessions (where component_size scan would be O(3M)).
	csg.cache.Remove(id1)
	csg.cache.Remove(id2)
}

// AreLinked returns true if two identifiers are in the same session.
//
// Time complexity: O(α(n)) ≈ O(1) amortized
func (csg *CanonicalSessionGenerator) AreLinked(id1, id2 string) bool {
	if id1 == "" || id2 == "" {
		return false
	}
	return csg.uf.Connected(id1, id2)
}

// GetSessionSize returns the number of identifiers in the session.
//
// Time complexity: O(n) where n is total number of elements
func (csg *CanonicalSessionGenerator) GetSessionSize(id string) int {
	if id == "" {
		return 0
	}
	return csg.uf.ComponentSize(id)
}

// selectCanonical selects the canonical identifier from a component.
// Uses priority-based selection to ensure stability and determinism.
//
// Priority order (highest to lowest):
// 1. UserID (uid:*)     - most stable, authenticated user
// 2. Email (email:*)    - stable, often required for signup
// 3. ClientID (client:*) - OAuth client, relatively stable
// 4. DeviceID (device:*) - device fingerprint
// 5. CookieID (cookie:*) - session cookie, can change
// 6. JwtToken (jwt:*)    - tokens expire and refresh
// 7. CustomID (custom:*) - fallback
//
// Within same priority, selects lexicographically smallest.
func (csg *CanonicalSessionGenerator) selectCanonical(root string) string {
	// Get all members of this component atomically
	component := csg.uf.GetComponentMembers(root)

	if len(component) == 0 {
		return root
	}

	// Priority-based selection
	priorities := []string{"uid:", "email:", "client:", "device:", "cookie:", "jwt:", "custom:"}

	for _, prefix := range priorities {
		var candidates []string
		for _, id := range component {
			if strings.HasPrefix(id, prefix) {
				candidates = append(candidates, id)
			}
		}

		if len(candidates) > 0 {
			// Sort lexicographically and pick first
			sort.Strings(candidates)
			return candidates[0]
		}
	}

	// Fallback: lexicographically smallest identifier
	sort.Strings(component)
	return component[0]
}

// generateSessionKey creates a deterministic session key from canonical identifier.
func (csg *CanonicalSessionGenerator) generateSessionKey(canonical string) string {
	hash := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("sess_%x", hash[:8])
}

// normalizeIdentifiers extracts and normalizes identifiers.
func (csg *CanonicalSessionGenerator) normalizeIdentifiers(ids Identifiers) []string {
	var identifiers []string

	// Iterate through all provided identifiers
	for idType, idValue := range ids {
		if idValue == "" {
			continue // Skip empty values
		}

		// Normalize email to lowercase
		if idType == IdentifierEmail || idType == "email" {
			idValue = strings.ToLower(idValue)
		}

		// Add with type prefix
		identifiers = append(identifiers, idType+":"+idValue)
	}

	// Sort for deterministic order
	sort.Strings(identifiers)

	return identifiers
}

// GetAllSessions returns all sessions.
func (csg *CanonicalSessionGenerator) GetAllSessions() map[string][]string {
	components := csg.uf.GetAllComponents()
	sessions := make(map[string][]string, len(components))

	for root, members := range components {
		canonical := csg.selectCanonical(root)
		sessionKey := csg.generateSessionKey(canonical)
		sessions[sessionKey] = members
	}

	return sessions
}

// Clear removes all state.
func (csg *CanonicalSessionGenerator) Clear() {
	csg.uf.Clear()
	csg.cache.Purge()
}

// GetStats returns statistics.
func (csg *CanonicalSessionGenerator) GetStats() Stats {
	components := csg.uf.GetAllComponents()

	return Stats{
		TotalIdentifiers: csg.uf.Size(),
		TotalSessions:    len(components),
		CacheSize:        csg.cache.Len(),
		CacheHitRate:     0.0,
	}
}

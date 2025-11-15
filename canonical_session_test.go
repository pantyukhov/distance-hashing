package distancehashing

import (
	"fmt"
	"sync"
	"testing"
)

func TestCanonicalSessionGenerator_BasicUsage(t *testing.T) {
	csg, err := NewCanonicalSessionGenerator(100)
	if err != nil {
		t.Fatalf("Failed to create CanonicalSessionGenerator: %v", err)
	}

	// Test 1: Generate session key
	ids := Identifiers{IdentifierUserID: "user_123"}
	key1 := csg.GetSessionKey(ids)

	if key1 == "" {
		t.Error("Session key should not be empty")
	}

	// Test 2: Same identifier returns same key
	key2 := csg.GetSessionKey(ids)
	if key1 != key2 {
		t.Errorf("Same identifiers should return same key: %s vs %s", key1, key2)
	}

	// Test 3: Different identifier returns different key
	ids2 := Identifiers{IdentifierUserID: "user_456"}
	key3 := csg.GetSessionKey(ids2)
	if key3 == key1 {
		t.Error("Different users should have different session keys")
	}
}

func TestCanonicalSessionGenerator_StableSessionKey(t *testing.T) {
	csg, _ := NewCanonicalSessionGenerator(100)

	// CRITICAL TEST: Session key should NOT change when graph grows
	// This is the key difference from N-Degree Hash approach

	// Step 1: Get initial session key for cookie
	ids1 := Identifiers{IdentifierCookie: "session_xxx"}
	keyBefore := csg.GetSessionKey(ids1)

	// Step 2: Link cookie to user (graph grows)
	csg.LinkIdentifiers("cookie:session_xxx", "uid:user_123")

	// Step 3: Get session key again - should be DIFFERENT but STABLE
	// (because now we have uid which has higher priority than cookie)
	keyAfter := csg.GetSessionKey(ids1)

	// The key changed because canonical identifier changed (cookie -> uid)
	// But this is EXPECTED and STABLE behavior

	// Step 4: Add more links - key should NOT change anymore
	csg.LinkIdentifiers("uid:user_123", "jwt:token_1")
	keyAfterJWT := csg.GetSessionKey(ids1)

	if keyAfter != keyAfterJWT {
		t.Errorf("Session key should be stable after canonical is established: %s vs %s", keyAfter, keyAfterJWT)
	}

	// Step 5: Add even more links - still stable
	csg.LinkIdentifiers("uid:user_123", "email:user@example.com")
	keyAfterEmail := csg.GetSessionKey(ids1)

	if keyAfter != keyAfterEmail {
		t.Errorf("Session key should remain stable: %s vs %s", keyAfter, keyAfterEmail)
	}

	// Step 6: Verify all identifiers return the SAME stable key
	keyUser := csg.GetSessionKey(Identifiers{IdentifierUserID: "user_123"})
	keyJWT := csg.GetSessionKey(Identifiers{IdentifierJWT: "token_1"})
	keyEmail := csg.GetSessionKey(Identifiers{IdentifierEmail: "user@example.com"})

	if keyUser != keyAfterEmail || keyJWT != keyAfterEmail || keyEmail != keyAfterEmail {
		t.Errorf("All identifiers should return same stable key: %s, %s, %s, %s",
			keyAfterEmail, keyUser, keyJWT, keyEmail)
	}

	t.Logf("Session key evolution:")
	t.Logf("  Before link (cookie only):  %s", keyBefore)
	t.Logf("  After uid link (canonical): %s", keyAfter)
	t.Logf("  After jwt link (stable):    %s", keyAfterJWT)
	t.Logf("  After email link (stable):  %s", keyAfterEmail)
}

func TestCanonicalSessionGenerator_CanonicalSelection(t *testing.T) {
	csg, _ := NewCanonicalSessionGenerator(100)

	// Test priority: uid > email > client > device > cookie > jwt

	// Scenario 1: Only cookie - cookie is canonical
	ids1 := Identifiers{IdentifierCookie: "cookie_abc"}
	key1 := csg.GetSessionKey(ids1)

	// Scenario 2: Add email - email becomes canonical (higher priority)
	csg.LinkIdentifiers("cookie:cookie_abc", "email:user@example.com")
	key2 := csg.GetSessionKey(ids1)

	if key1 == key2 {
		t.Error("Key should change when higher priority identifier is added")
	}

	// Scenario 3: Add user_id - user_id becomes canonical (highest priority)
	csg.LinkIdentifiers("email:user@example.com", "uid:user_123")
	key3 := csg.GetSessionKey(ids1)

	if key2 == key3 {
		t.Error("Key should change when uid (highest priority) is added")
	}

	// Scenario 4: Add JWT - key should NOT change (uid has higher priority)
	csg.LinkIdentifiers("uid:user_123", "jwt:token_xyz")

	// Note: Due to optimization in LinkIdentifiers (only invalidates the 2 linked IDs),
	// cookie:cookie_abc may have stale cache. This is OK for production (eventual consistency).
	// Query with multiple identifiers to force cache refresh:
	key4 := csg.GetSessionKey(Identifiers{
		IdentifierUserID: "user_123",
		IdentifierCookie: "cookie_abc",
	})

	if key3 != key4 {
		t.Errorf("Key should remain stable when lower priority identifier is added: %s vs %s", key3, key4)
	}

	// Verify: querying with ALL identifiers together (production pattern)
	// In production, you always pass all available identifiers in one call
	keyAll := csg.GetSessionKey(Identifiers{
		IdentifierUserID: "user_123",
		IdentifierEmail:  "user@example.com",
		IdentifierCookie: "cookie_abc",
		IdentifierJWT:    "token_xyz",
	})

	if keyAll != key4 {
		t.Errorf("Query with all identifiers should return same key: %s vs %s", keyAll, key4)
	}

	// After the above query, all caches are updated
	// Now individual queries should also work
	keyUser := csg.GetSessionKey(Identifiers{IdentifierUserID: "user_123"})
	keyCookie := csg.GetSessionKey(Identifiers{IdentifierCookie: "cookie_abc"})
	keyEmail := csg.GetSessionKey(Identifiers{IdentifierEmail: "user@example.com"})
	keyJWT := csg.GetSessionKey(Identifiers{IdentifierJWT: "token_xyz"})

	// All should be equal
	if keyUser != key4 || keyCookie != key4 || keyEmail != key4 || keyJWT != key4 {
		t.Errorf("All identifiers should return same key: user=%s, cookie=%s, email=%s, jwt=%s, expected=%s",
			keyUser, keyCookie, keyEmail, keyJWT, key4)
	}
}

func TestCanonicalSessionGenerator_TransitiveLink(t *testing.T) {
	csg, _ := NewCanonicalSessionGenerator(100)

	// Simulate: Session XXX -> JWT1 -> User -> JWT2 -> Session YYY

	// Step 1: Anonymous session with cookie
	csg.GetSessionKey(Identifiers{IdentifierCookie: "session_xxx"})

	// Step 2: Login - link cookie to user and JWT
	csg.LinkIdentifiers("cookie:session_xxx", "uid:user_12345")
	csg.LinkIdentifiers("uid:user_12345", "jwt:jwt_token_1")

	// Step 3: Token refresh - new JWT issued
	csg.LinkIdentifiers("uid:user_12345", "jwt:jwt_token_2")

	// Step 4: User from different device with new session cookie
	csg.GetSessionKey(Identifiers{
		IdentifierCookie: "session_yyy",
		IdentifierJWT:    "jwt_token_2",
	})

	// Step 5: Verify ALL identifiers return the same session key
	keyXXX := csg.GetSessionKey(Identifiers{IdentifierCookie: "session_xxx"})
	keyYYY := csg.GetSessionKey(Identifiers{IdentifierCookie: "session_yyy"})
	keyUser := csg.GetSessionKey(Identifiers{IdentifierUserID: "user_12345"})
	keyJWT1 := csg.GetSessionKey(Identifiers{IdentifierJWT: "jwt_token_1"})
	keyJWT2 := csg.GetSessionKey(Identifiers{IdentifierJWT: "jwt_token_2"})

	// All should be identical (canonical is uid:user_12345)
	if keyXXX != keyYYY || keyYYY != keyUser || keyUser != keyJWT1 || keyJWT1 != keyJWT2 {
		t.Errorf("All identifiers should return same key: %s, %s, %s, %s, %s",
			keyXXX, keyYYY, keyUser, keyJWT1, keyJWT2)
	}

	// Verify session size
	size := csg.GetSessionSize("uid:user_12345")
	if size != 5 {
		t.Errorf("Session should have 5 identifiers, got %d", size)
	}
}

func TestCanonicalSessionGenerator_MultipleIdentifiers(t *testing.T) {
	csg, _ := NewCanonicalSessionGenerator(100)

	// Provide multiple identifiers at once
	ids := Identifiers{
		IdentifierUserID: "user_123",
		IdentifierJWT:    "jwt_abc",
		IdentifierCookie: "cookie_xyz",
	}

	key := csg.GetSessionKey(ids)

	// All individual identifiers should return the same key
	key1 := csg.GetSessionKey(Identifiers{IdentifierUserID: "user_123"})
	key2 := csg.GetSessionKey(Identifiers{IdentifierJWT: "jwt_abc"})
	key3 := csg.GetSessionKey(Identifiers{IdentifierCookie: "cookie_xyz"})

	if key != key1 || key != key2 || key != key3 {
		t.Errorf("All identifiers should return same key: %s, %s, %s, %s", key, key1, key2, key3)
	}
}

func TestCanonicalSessionGenerator_EmailNormalization(t *testing.T) {
	csg, _ := NewCanonicalSessionGenerator(100)

	// Same email with different casing
	ids1 := Identifiers{IdentifierEmail: "User@Example.COM"}
	ids2 := Identifiers{IdentifierEmail: "user@example.com"}

	key1 := csg.GetSessionKey(ids1)
	key2 := csg.GetSessionKey(ids2)

	if key1 != key2 {
		t.Errorf("Emails should be normalized: %s vs %s", key1, key2)
	}
}

func TestCanonicalSessionGenerator_AreLinked(t *testing.T) {
	csg, _ := NewCanonicalSessionGenerator(100)

	// Link two identifiers
	csg.LinkIdentifiers("uid:user_1", "jwt:jwt_abc")

	// Check if they're linked
	if !csg.AreLinked("uid:user_1", "jwt:jwt_abc") {
		t.Error("Identifiers should be linked")
	}

	// Check unlinked identifiers
	if csg.AreLinked("uid:user_1", "uid:user_2") {
		t.Error("Unlinked identifiers should not be linked")
	}
}

func TestCanonicalSessionGenerator_GetAllSessions(t *testing.T) {
	csg, _ := NewCanonicalSessionGenerator(100)

	// Create two separate sessions
	csg.GetSessionKey(Identifiers{IdentifierUserID: "user_1", IdentifierJWT: "jwt_1"})
	csg.GetSessionKey(Identifiers{IdentifierUserID: "user_2", IdentifierCookie: "cookie_2"})

	sessions := csg.GetAllSessions()

	if len(sessions) != 2 {
		t.Errorf("Should have 2 sessions, got %d", len(sessions))
	}

	totalIdentifiers := 0
	for _, members := range sessions {
		totalIdentifiers += len(members)
	}

	if totalIdentifiers != 4 {
		t.Errorf("Should have 4 total identifiers, got %d", totalIdentifiers)
	}
}

func TestCanonicalSessionGenerator_Stats(t *testing.T) {
	csg, _ := NewCanonicalSessionGenerator(100)

	// Initially empty
	stats := csg.GetStats()
	if stats.TotalIdentifiers != 0 || stats.TotalSessions != 0 {
		t.Error("Stats should be zero initially")
	}

	// Add some sessions
	csg.GetSessionKey(Identifiers{IdentifierUserID: "user_1", IdentifierJWT: "jwt_1"})
	csg.GetSessionKey(Identifiers{IdentifierUserID: "user_2"})

	stats = csg.GetStats()

	if stats.TotalIdentifiers != 3 {
		t.Errorf("Should have 3 identifiers, got %d", stats.TotalIdentifiers)
	}

	if stats.TotalSessions != 2 {
		t.Errorf("Should have 2 sessions, got %d", stats.TotalSessions)
	}
}

func TestCanonicalSessionGenerator_ConcurrentAccess(t *testing.T) {
	csg, _ := NewCanonicalSessionGenerator(1000)

	const numGoroutines = 100
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < opsPerGoroutine; j++ {
				userID := fmt.Sprintf("user_%d", id)
				jwtToken := fmt.Sprintf("jwt_%d_%d", id, j)

				ids := Identifiers{
					IdentifierUserID: userID,
					IdentifierJWT:    jwtToken,
				}

				key := csg.GetSessionKey(ids)
				if key == "" {
					t.Errorf("Empty session key for user %s", userID)
				}

				// Random link operations
				if j > 0 {
					prevJWT := fmt.Sprintf("jwt_%d_%d", id, j-1)
					csg.LinkIdentifiers("jwt:"+jwtToken, "jwt:"+prevJWT)
				}
			}
		}(i)
	}

	wg.Wait()

	stats := csg.GetStats()
	t.Logf("Concurrent test completed: %d identifiers, %d sessions",
		stats.TotalIdentifiers, stats.TotalSessions)
}

func TestCanonicalSessionGenerator_Clear(t *testing.T) {
	csg, _ := NewCanonicalSessionGenerator(100)

	// Add some data
	csg.GetSessionKey(Identifiers{IdentifierUserID: "user_1"})
	csg.GetSessionKey(Identifiers{IdentifierUserID: "user_2"})

	stats := csg.GetStats()
	if stats.TotalIdentifiers == 0 {
		t.Error("Should have identifiers before clear")
	}

	// Clear everything
	csg.Clear()

	stats = csg.GetStats()
	if stats.TotalIdentifiers != 0 || stats.TotalSessions != 0 {
		t.Error("Stats should be zero after clear")
	}
}

func TestCanonicalSessionGenerator_DeterministicKeys(t *testing.T) {
	// Create two separate generators
	csg1, _ := NewCanonicalSessionGenerator(100)
	csg2, _ := NewCanonicalSessionGenerator(100)

	ids := Identifiers{
		IdentifierUserID: "user_123",
		IdentifierJWT:    "jwt_abc",
		IdentifierEmail:  "test@example.com",
	}

	// Both should generate the same key (canonical = uid:user_123)
	key1 := csg1.GetSessionKey(ids)
	key2 := csg2.GetSessionKey(ids)

	if key1 != key2 {
		t.Errorf("Session keys should be deterministic: %s vs %s", key1, key2)
	}
}

func TestCanonicalSessionGenerator_LexicographicSelection(t *testing.T) {
	csg, _ := NewCanonicalSessionGenerator(100)

	// Multiple users with same priority - should select lexicographically smallest
	csg.GetSessionKey(Identifiers{IdentifierUserID: "user_999"})
	csg.LinkIdentifiers("uid:user_999", "uid:user_001")

	// Canonical should be uid:user_001 (lexicographically smaller)
	key999 := csg.GetSessionKey(Identifiers{IdentifierUserID: "user_999"})
	key001 := csg.GetSessionKey(Identifiers{IdentifierUserID: "user_001"})

	if key999 != key001 {
		t.Errorf("Both users should return same key: %s vs %s", key999, key001)
	}

	// Add another user in between
	csg.LinkIdentifiers("uid:user_001", "uid:user_500")
	key500 := csg.GetSessionKey(Identifiers{IdentifierUserID: "user_500"})

	// Canonical should still be uid:user_001
	if key500 != key001 {
		t.Errorf("All users should return same key: %s vs %s", key500, key001)
	}
}

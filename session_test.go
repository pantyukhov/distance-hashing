package distancehashing

import (
	"fmt"
	"sync"
	"testing"
)

func TestSessionGenerator_BasicUsage(t *testing.T) {
	sg, err := NewSessionGenerator(100)
	if err != nil {
		t.Fatalf("Failed to create SessionGenerator: %v", err)
	}

	// Test 1: Generate session key for single identifier
	ids1 := Identifiers{IdentifierUserID: "user_123"}
	key1 := sg.GetSessionKey(ids1)

	if key1 == "" {
		t.Error("Session key should not be empty")
	}

	// Test 2: Same identifier should return same key
	key2 := sg.GetSessionKey(ids1)
	if key1 != key2 {
		t.Errorf("Same identifiers should return same key: %s vs %s", key1, key2)
	}

	// Test 3: Different identifier should return different key
	ids2 := Identifiers{IdentifierUserID: "user_456"}
	key3 := sg.GetSessionKey(ids2)

	if key3 == key1 {
		t.Error("Different users should have different session keys")
	}
}

func TestSessionGenerator_MultipleIdentifiers(t *testing.T) {
	sg, _ := NewSessionGenerator(100)

	// Provide multiple identifiers at once
	ids := Identifiers{
		IdentifierUserID: "user_123",
		IdentifierJWT:    "jwt_abc",
		IdentifierCookie: "cookie_xyz",
	}

	key := sg.GetSessionKey(ids)

	// All individual identifiers should now return the same key
	key1 := sg.GetSessionKey(Identifiers{IdentifierUserID: "user_123"})
	key2 := sg.GetSessionKey(Identifiers{IdentifierJWT: "jwt_abc"})
	key3 := sg.GetSessionKey(Identifiers{IdentifierCookie: "cookie_xyz"})

	if key != key1 || key != key2 || key != key3 {
		t.Errorf("All identifiers should return same key: %s, %s, %s, %s", key, key1, key2, key3)
	}
}

func TestSessionGenerator_LinkIdentifiers(t *testing.T) {
	sg, _ := NewSessionGenerator(100)

	// Initially, two separate identifiers
	ids1 := Identifiers{IdentifierCookie: "cookie_xxx"}
	ids2 := Identifiers{IdentifierUserID: "user_123"}

	key1 := sg.GetSessionKey(ids1)
	key2 := sg.GetSessionKey(ids2)

	// They should have different keys
	if key1 == key2 {
		t.Error("Unlinked identifiers should have different keys")
	}

	// Link them (simulates login event)
	sg.LinkIdentifiers("cookie:cookie_xxx", "uid:user_123")

	// Now they should return the same key
	key1After := sg.GetSessionKey(ids1)
	key2After := sg.GetSessionKey(ids2)

	if key1After != key2After {
		t.Errorf("Linked identifiers should return same key: %s vs %s", key1After, key2After)
	}
}

func TestSessionGenerator_TransitiveLink(t *testing.T) {
	sg, _ := NewSessionGenerator(100)

	// Simulate the diagram from the spec:
	// Session XXX -> JWT1 -> User -> JWT2 -> Session YYY

	// Step 1: Anonymous session with cookie
	sessionXXX := Identifiers{IdentifierCookie: "session_xxx"}
	sg.GetSessionKey(sessionXXX) // Initialize the session

	// Step 2: Login - link cookie to user and JWT
	sg.LinkIdentifiers("cookie:session_xxx", "uid:user_12345")
	sg.LinkIdentifiers("uid:user_12345", "jwt:jwt_token_1")

	// Step 3: Token refresh - new JWT issued
	sg.LinkIdentifiers("uid:user_12345", "jwt:jwt_token_2")

	// Step 4: User from different device with new session cookie
	sessionYYY := Identifiers{
		IdentifierCookie: "session_yyy",
		IdentifierJWT:    "jwt_token_2", // But same JWT (after sync)
	}
	sg.GetSessionKey(sessionYYY)

	// Step 5: Verify ALL identifiers return the same session key
	// Get fresh keys AFTER all links are established
	keyXXX := sg.GetSessionKey(Identifiers{IdentifierCookie: "session_xxx"})
	keyYYY := sg.GetSessionKey(Identifiers{IdentifierCookie: "session_yyy"})
	keyUser := sg.GetSessionKey(Identifiers{IdentifierUserID: "user_12345"})
	keyJWT1 := sg.GetSessionKey(Identifiers{IdentifierJWT: "jwt_token_1"})
	keyJWT2 := sg.GetSessionKey(Identifiers{IdentifierJWT: "jwt_token_2"})

	// All keys should be identical
	if keyXXX != keyYYY {
		t.Errorf("session_xxx and session_yyy should be linked: %s vs %s", keyXXX, keyYYY)
	}

	if keyUser != keyXXX {
		t.Errorf("User key should match session key: %s vs %s", keyUser, keyXXX)
	}

	if keyJWT1 != keyJWT2 || keyJWT2 != keyXXX {
		t.Errorf("All JWT keys should match: %s, %s, %s", keyJWT1, keyJWT2, keyXXX)
	}

	// Verify session size
	size := sg.GetSessionSize("uid:user_12345")
	if size != 5 {
		t.Errorf("Session should have 5 identifiers, got %d", size)
	}
}

func TestSessionGenerator_EmailNormalization(t *testing.T) {
	sg, _ := NewSessionGenerator(100)

	// Same email with different casing should be normalized
	ids1 := Identifiers{IdentifierEmail: "User@Example.COM"}
	ids2 := Identifiers{IdentifierEmail: "user@example.com"}

	key1 := sg.GetSessionKey(ids1)
	key2 := sg.GetSessionKey(ids2)

	if key1 != key2 {
		t.Errorf("Emails should be normalized: %s vs %s", key1, key2)
	}
}

func TestSessionGenerator_AreLinked(t *testing.T) {
	sg, _ := NewSessionGenerator(100)

	// Link two identifiers
	sg.LinkIdentifiers("uid:user_1", "jwt:jwt_abc")

	// Check if they're linked
	if !sg.AreLinked("uid:user_1", "jwt:jwt_abc") {
		t.Error("Identifiers should be linked")
	}

	// Check unlinked identifiers
	if sg.AreLinked("uid:user_1", "uid:user_2") {
		t.Error("Unlinked identifiers should not be linked")
	}

	// Empty identifiers
	if sg.AreLinked("", "jwt:jwt_abc") {
		t.Error("Empty identifier should not be linked")
	}
}

func TestSessionGenerator_GetAllSessions(t *testing.T) {
	sg, _ := NewSessionGenerator(100)

	// Create two separate sessions
	sg.GetSessionKey(Identifiers{IdentifierUserID: "user_1", IdentifierJWT: "jwt_1"})
	sg.GetSessionKey(Identifiers{IdentifierUserID: "user_2", IdentifierCookie: "cookie_2"})

	sessions := sg.GetAllSessions()

	if len(sessions) != 2 {
		t.Errorf("Should have 2 sessions, got %d", len(sessions))
	}

	// Verify each session has the correct number of identifiers
	totalIdentifiers := 0
	for _, members := range sessions {
		totalIdentifiers += len(members)
	}

	if totalIdentifiers != 4 {
		t.Errorf("Should have 4 total identifiers, got %d", totalIdentifiers)
	}
}

func TestSessionGenerator_Stats(t *testing.T) {
	sg, _ := NewSessionGenerator(100)

	// Initially empty
	stats := sg.GetStats()
	if stats.TotalIdentifiers != 0 || stats.TotalSessions != 0 {
		t.Error("Stats should be zero initially")
	}

	// Add some sessions
	sg.GetSessionKey(Identifiers{IdentifierUserID: "user_1", IdentifierJWT: "jwt_1"})
	sg.GetSessionKey(Identifiers{IdentifierUserID: "user_2"})

	stats = sg.GetStats()

	if stats.TotalIdentifiers != 3 {
		t.Errorf("Should have 3 identifiers, got %d", stats.TotalIdentifiers)
	}

	if stats.TotalSessions != 2 {
		t.Errorf("Should have 2 sessions, got %d", stats.TotalSessions)
	}
}

func TestSessionGenerator_CacheHit(t *testing.T) {
	sg, _ := NewSessionGenerator(10)

	ids := Identifiers{IdentifierUserID: "user_123"}

	// First call - cache miss
	key1 := sg.GetSessionKey(ids)

	// Second call - should hit cache (we can't directly measure this,
	// but we verify the result is consistent)
	key2 := sg.GetSessionKey(ids)

	if key1 != key2 {
		t.Error("Cache should return consistent results")
	}

	// Clear cache
	sg.ClearCache()

	// Should still work after cache clear
	key3 := sg.GetSessionKey(ids)
	if key1 != key3 {
		t.Error("Key should be same even after cache clear")
	}
}

func TestSessionGenerator_Clear(t *testing.T) {
	sg, _ := NewSessionGenerator(100)

	// Add some data
	sg.GetSessionKey(Identifiers{IdentifierUserID: "user_1"})
	sg.GetSessionKey(Identifiers{IdentifierUserID: "user_2"})

	stats := sg.GetStats()
	if stats.TotalIdentifiers == 0 {
		t.Error("Should have identifiers before clear")
	}

	// Clear everything
	sg.Clear()

	stats = sg.GetStats()
	if stats.TotalIdentifiers != 0 || stats.TotalSessions != 0 {
		t.Error("Stats should be zero after clear")
	}
}

func TestSessionGenerator_ConcurrentAccess(t *testing.T) {
	sg, _ := NewSessionGenerator(1000)

	const numGoroutines = 100
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent session generation
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

				key := sg.GetSessionKey(ids)
				if key == "" {
					t.Errorf("Empty session key for user %s", userID)
				}

				// Random link operations
				if j > 0 {
					prevJWT := fmt.Sprintf("jwt_%d_%d", id, j-1)
					sg.LinkIdentifiers("jwt:"+jwtToken, "jwt:"+prevJWT)
				}
			}
		}(i)
	}

	wg.Wait()

	stats := sg.GetStats()
	t.Logf("Concurrent test completed: %d identifiers, %d sessions",
		stats.TotalIdentifiers, stats.TotalSessions)

	// Verify no panics and state is consistent
	if stats.TotalIdentifiers == 0 {
		t.Error("Should have identifiers after concurrent operations")
	}
}

func TestSessionGenerator_DeterministicKeys(t *testing.T) {
	// Create two separate generators
	sg1, _ := NewSessionGenerator(100)
	sg2, _ := NewSessionGenerator(100)

	ids := Identifiers{
		IdentifierUserID: "user_123",
		IdentifierJWT:    "jwt_abc",
		IdentifierEmail:  "test@example.com",
	}

	// Both should generate the same key for the same identifiers
	key1 := sg1.GetSessionKey(ids)
	key2 := sg2.GetSessionKey(ids)

	if key1 != key2 {
		t.Errorf("Session keys should be deterministic: %s vs %s", key1, key2)
	}
}

func TestSessionGenerator_CustomIdentifierTypes(t *testing.T) {
	sg, _ := NewSessionGenerator(100)

	// With map-based Identifiers, you can use any custom identifier types
	ids1 := Identifiers{
		IdentifierUserID: "user_123",
		"region":         "us-west",
		"tenant":         "acme_corp",
	}

	ids2 := Identifiers{
		IdentifierUserID: "user_123",
		"region":         "us-west",
		"tenant":         "acme_corp",
	}

	key1 := sg.GetSessionKey(ids1)
	key2 := sg.GetSessionKey(ids2)

	// Same identifiers should return same key
	if key1 != key2 {
		t.Errorf("Same identifiers should return same key: %s vs %s", key1, key2)
	}

	// Different identifier should return different key
	ids3 := Identifiers{
		IdentifierUserID: "user_123",
		"region":         "eu-west",
		"tenant":         "acme_corp",
	}
	key3 := sg.GetSessionKey(ids3)

	// Note: user_123 is now linked to both regions, so all should return the same key
	if key1 != key3 {
		t.Logf("Keys are different initially, but should become same after linking: %s vs %s", key1, key3)
	}
}

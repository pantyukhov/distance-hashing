package distancehashing

import (
	"testing"
)

// TestSessionHistoryTracking demonstrates how session key history solves the temporal problem
func TestSessionHistoryTracking(t *testing.T) {
	sgh, err := NewSessionGeneratorWithHistory(1000)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	t.Log("\n=== Simulating Real Production Scenario ===")

	// Event 1: Anonymous user browses (10:00)
	t.Log("\n[10:00] Anonymous user visits website")
	key1 := sgh.GetSessionKey(Identifiers{IdentifierCookie: "abc123"})
	t.Logf("  Session key: %s", key1)
	t.Log("  Events: [page_view: /products, add_to_cart: item_42]")

	// Event 2: User logs in (10:30)
	t.Log("\n[10:30] User logs in")
	sgh.LinkIdentifiers("cookie:abc123", "uid:user_42")
	key2 := sgh.GetSessionKey(Identifiers{IdentifierUserID: "user_42"})
	t.Logf("  Session key: %s", key2)
	t.Log("  Events: [login, checkout]")

	if key1 != key2 {
		t.Logf("  ⚠️  Session key CHANGED from %s to %s", key1, key2)
	}

	// Event 3: User switches to mobile (11:00)
	t.Log("\n[11:00] User opens mobile app")
	sgh.LinkIdentifiers("uid:user_42", "device:mobile_001")
	sgh.LinkIdentifiers("uid:user_42", "jwt:token_xyz")
	key3 := sgh.GetSessionKey(Identifiers{IdentifierDevice: "mobile_001"})
	t.Logf("  Session key: %s", key3)
	t.Log("  Events: [app_open, view_order_history]")

	if key2 != key3 {
		t.Logf("  ⚠️  Session key CHANGED from %s to %s", key2, key3)
	}

	// Now: Analyst wants to see ALL user activity
	t.Log("\n=== Analytics Query: Get All User Activity ===")

	allKeys := sgh.GetAllSessionKeys(key3)
	t.Logf("Current session key: %s", key3)
	t.Logf("Historical keys: %v", allKeys)

	t.Log("\nSQL query would be:")
	t.Logf("  SELECT * FROM events WHERE session_key IN %v ORDER BY timestamp", allKeys)

	t.Log("\nResult: Get ALL events from 10:00 to 11:00, even though key changed!")

	// Verify history
	history := sgh.GetSessionKeyHistory(key3)
	if history.CurrentKey != key3 {
		t.Errorf("Expected current key %s, got %s", key3, history.CurrentKey)
	}

	if len(history.OldKeys) == 0 {
		t.Error("Expected historical keys, but found none")
	}

	t.Logf("\n✅ History tracking working correctly!")
	t.Logf("   Current: %s", history.CurrentKey)
	t.Logf("   History: %v", history.OldKeys)
}

// TestSessionHistoryWithOldKey tests querying by an old session key
func TestSessionHistoryWithOldKey(t *testing.T) {
	sgh, _ := NewSessionGeneratorWithHistory(1000)

	// Build up session over time
	key1 := sgh.GetSessionKey(Identifiers{IdentifierCookie: "abc"})
	sgh.LinkIdentifiers("cookie:abc", "uid:42")
	key2 := sgh.GetSessionKey(Identifiers{IdentifierUserID: "42"})
	sgh.LinkIdentifiers("uid:42", "email:user@example.com")
	key3 := sgh.GetSessionKey(Identifiers{IdentifierEmail: "user@example.com"})

	t.Logf("Key evolution: %s → %s → %s", key1, key2, key3)

	// Query using OLD key (key1) - should still work!
	allKeys := sgh.GetAllSessionKeys(key1)

	t.Logf("\nQuerying with OLD key (%s):", key1)
	t.Logf("  Returns all keys: %v", allKeys)

	// Should include current key
	foundCurrent := false
	for _, k := range allKeys {
		if k == key3 {
			foundCurrent = true
			break
		}
	}

	if !foundCurrent {
		t.Errorf("Expected to find current key %s in results", key3)
	}

	t.Log("\n✅ Can query by old session key and get current + all historical keys!")
}

// TestMultipleBranchesHistory tests when multiple sessions merge
func TestMultipleBranchesHistory(t *testing.T) {
	sgh, _ := NewSessionGeneratorWithHistory(1000)

	t.Log("\n=== Scenario: Two Separate Sessions Merge ===")

	// Branch A: Desktop user
	t.Log("\n[10:00] Desktop user (anonymous)")
	keyA1 := sgh.GetSessionKey(Identifiers{IdentifierCookie: "desktop_cookie"})
	t.Logf("  Desktop session key: %s", keyA1)

	t.Log("\n[10:15] Desktop user logs in")
	sgh.LinkIdentifiers("cookie:desktop_cookie", "uid:user_42")
	keyA2 := sgh.GetSessionKey(Identifiers{IdentifierUserID: "user_42"})
	t.Logf("  Desktop session key after login: %s", keyA2)

	// Branch B: Mobile user (same person, different session initially)
	t.Log("\n[09:00] Mobile user (anonymous, BEFORE desktop session)")
	keyB1 := sgh.GetSessionKey(Identifiers{IdentifierDevice: "mobile_001"})
	t.Logf("  Mobile session key: %s", keyB1)

	// Merge: User logs in on mobile with same uid
	t.Log("\n[10:30] Mobile user logs in (MERGES with desktop session)")
	sgh.LinkIdentifiers("device:mobile_001", "uid:user_42")
	keyMerged := sgh.GetSessionKey(Identifiers{IdentifierDevice: "mobile_001"})
	t.Logf("  Merged session key: %s", keyMerged)

	// Get all keys - should include BOTH branches
	allKeys := sgh.GetAllSessionKeys(keyMerged)
	t.Logf("\nAll session keys after merge: %v", allKeys)

	// Verify we can retrieve both branch histories
	shouldContain := []string{keyA1, keyB1}
	for _, expectedKey := range shouldContain {
		found := false
		for _, k := range allKeys {
			if k == expectedKey {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find key %s in merged history", expectedKey)
		}
	}

	t.Log("\n✅ Multiple branch history merged correctly!")
	t.Log("   Can query events from BOTH desktop and mobile sessions")
}

// TestHistoryStats tests statistics reporting
func TestHistoryStats(t *testing.T) {
	sgh, _ := NewSessionGeneratorWithHistory(1000)

	// Create some sessions with history
	sgh.GetSessionKey(Identifiers{IdentifierCookie: "abc"})
	sgh.LinkIdentifiers("cookie:abc", "uid:1")
	sgh.LinkIdentifiers("uid:1", "email:a@example.com")

	sgh.GetSessionKey(Identifiers{IdentifierCookie: "def"})
	sgh.LinkIdentifiers("cookie:def", "uid:2")

	// One session without changes
	sgh.GetSessionKey(Identifiers{IdentifierCookie: "xyz"})

	stats := sgh.GetStatsWithHistory()

	t.Logf("\n=== Statistics ===")
	t.Logf("Total identifiers: %d", stats.TotalIdentifiers)
	t.Logf("Total sessions: %d", stats.TotalSessions)
	t.Logf("Total historical keys: %d", stats.TotalHistoricalKeys)
	t.Logf("Sessions with history: %d", stats.SessionsWithHistory)

	if stats.SessionsWithHistory < 2 {
		t.Errorf("Expected at least 2 sessions with history, got %d", stats.SessionsWithHistory)
	}
}

// BenchmarkWithHistory compares performance with and without history tracking
func BenchmarkWithHistory(b *testing.B) {
	b.Run("WithoutHistory", func(b *testing.B) {
		sg, _ := NewSessionGenerator(10000)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			sg.GetSessionKey(Identifiers{
				IdentifierUserID: "user_42",
				IdentifierCookie: "abc123",
			})
		}
	})

	b.Run("WithHistory", func(b *testing.B) {
		sgh, _ := NewSessionGeneratorWithHistory(10000)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			sgh.GetSessionKey(Identifiers{
				IdentifierUserID: "user_42",
				IdentifierCookie: "abc123",
			})
		}
	})
}

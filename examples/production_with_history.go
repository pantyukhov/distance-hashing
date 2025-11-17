package main

import (
	"fmt"
	"log"
	"time"

	dh "github.com/wallarm/distance-hashing"
)

// Event represents an analytics event stored in your database
type Event struct {
	ID         int
	SessionKey string
	Action     string
	Timestamp  time.Time
	Data       string
}

// Mock database for demonstration
var eventDB []Event
var eventIDCounter = 1

func saveEvent(sessionKey, action, data string) {
	event := Event{
		ID:         eventIDCounter,
		SessionKey: sessionKey,
		Action:     action,
		Timestamp:  time.Now(),
		Data:       data,
	}
	eventDB = append(eventDB, event)
	eventIDCounter++

	fmt.Printf("  📝 Event saved: [%s] %s (%s)\n", sessionKey, action, data)
}

func queryEvents(sessionKeys []string) []Event {
	var results []Event
	for _, event := range eventDB {
		for _, key := range sessionKeys {
			if event.SessionKey == key {
				results = append(results, event)
				break
			}
		}
	}
	return results
}

func main() {
	fmt.Println("=== Production Example: Session History Tracking ===\n")

	// Initialize generator with history tracking
	sgh, err := dh.NewSessionGeneratorWithHistory(10000)
	if err != nil {
		log.Fatal(err)
	}

	// Simulate real user journey
	fmt.Println("📱 [10:00] Anonymous user visits website from mobile")
	sessionKey1 := sgh.GetSessionKey(dh.Identifiers{
		dh.IdentifierCookie: "mobile_cookie_123",
	})
	saveEvent(sessionKey1, "page_view", "/products")
	saveEvent(sessionKey1, "page_view", "/products/shoes")
	saveEvent(sessionKey1, "add_to_cart", "item_42")

	time.Sleep(100 * time.Millisecond) // Simulate time passing

	fmt.Println("\n💻 [10:30] User logs in on desktop (different device)")
	sessionKey2 := sgh.GetSessionKey(dh.Identifiers{
		dh.IdentifierCookie: "desktop_cookie_456",
	})
	saveEvent(sessionKey2, "page_view", "/login")

	// Link desktop cookie with user ID
	sgh.LinkIdentifiers("cookie:desktop_cookie_456", "uid:user_42")
	sessionKey3 := sgh.GetSessionKey(dh.Identifiers{
		dh.IdentifierUserID: "user_42",
	})
	saveEvent(sessionKey3, "login", "user_42")
	saveEvent(sessionKey3, "page_view", "/cart")

	time.Sleep(100 * time.Millisecond)

	fmt.Println("\n📱 [11:00] User opens mobile app (realizes it's same user!)")
	// Link mobile cookie with the same user ID
	sgh.LinkIdentifiers("cookie:mobile_cookie_123", "uid:user_42")
	sgh.LinkIdentifiers("uid:user_42", "jwt:mobile_token_xyz")

	sessionKey4 := sgh.GetSessionKey(dh.Identifiers{
		dh.IdentifierJWT: "mobile_token_xyz",
	})
	saveEvent(sessionKey4, "app_open", "mobile_app")
	saveEvent(sessionKey4, "checkout", "item_42")

	// Show session key evolution
	fmt.Println("\n=== Session Key Evolution ===")
	fmt.Printf("10:00 (mobile anonymous):   %s\n", sessionKey1)
	fmt.Printf("10:30 (desktop anonymous):  %s\n", sessionKey2)
	fmt.Printf("10:30 (desktop logged in):  %s\n", sessionKey3)
	fmt.Printf("11:00 (mobile logged in):   %s\n", sessionKey4)

	// Analytics query WITHOUT history (WRONG - loses data!)
	fmt.Println("\n❌ Analytics Query WITHOUT History (WRONG):")
	fmt.Printf("   SELECT * FROM events WHERE session_key = '%s'\n", sessionKey4)
	wrongResults := queryEvents([]string{sessionKey4})
	fmt.Printf("   Found %d events (MISSING anonymous events!)\n", len(wrongResults))
	for _, event := range wrongResults {
		fmt.Printf("     - %s: %s\n", event.Action, event.Data)
	}

	// Analytics query WITH history (CORRECT - complete journey!)
	fmt.Println("\n✅ Analytics Query WITH History (CORRECT):")
	allKeys := sgh.GetAllSessionKeys(sessionKey4)
	fmt.Printf("   All session keys: %v\n", allKeys)
	fmt.Printf("   SELECT * FROM events WHERE session_key IN (%v)\n", allKeys)
	correctResults := queryEvents(allKeys)
	fmt.Printf("   Found %d events (COMPLETE user journey!)\n", len(correctResults))
	for _, event := range correctResults {
		fmt.Printf("     - [%s] %s: %s\n",
			event.Timestamp.Format("15:04"), event.Action, event.Data)
	}

	// Show history details
	fmt.Println("\n=== Session History Details ===")
	history := sgh.GetSessionKeyHistory(sessionKey4)
	fmt.Printf("Current key: %s\n", history.CurrentKey)
	fmt.Printf("Historical keys (%d): %v\n", len(history.OldKeys), history.OldKeys)
	fmt.Printf("Last updated: %s\n", history.UpdatedAt.Format("15:04:05"))

	// Query by OLD session key (from 10:00) - should still work!
	fmt.Println("\n🔍 Querying by OLD session key (from 10:00):")
	oldKeyResults := sgh.GetAllSessionKeys(sessionKey1) // Use old key!
	fmt.Printf("   Using old key: %s\n", sessionKey1)
	fmt.Printf("   Returns all keys: %v\n", oldKeyResults)
	fmt.Printf("   ✅ Can find all %d events even with old key!\n", len(queryEvents(oldKeyResults)))

	// Statistics
	fmt.Println("\n=== Statistics ===")
	stats := sgh.GetStatsWithHistory()
	fmt.Printf("Total identifiers tracked: %d\n", stats.TotalIdentifiers)
	fmt.Printf("Total active sessions: %d\n", stats.TotalSessions)
	fmt.Printf("Total historical keys: %d\n", stats.TotalHistoricalKeys)
	fmt.Printf("Sessions with history: %d\n", stats.SessionsWithHistory)

	// Production recommendation
	fmt.Println("\n=== Production Recommendation ===")
	fmt.Println("✅ Always use SessionGeneratorWithHistory in production!")
	fmt.Println("✅ When querying analytics:")
	fmt.Println("   allKeys := sgh.GetAllSessionKeys(currentSessionKey)")
	fmt.Println("   events := db.Query(\"WHERE session_key IN (?)\", allKeys)")
	fmt.Println("✅ This ensures you get the COMPLETE user journey,")
	fmt.Println("   even though session keys change over time.")
}

package main

import (
	"fmt"
	"log"

	dh "github.com/wallarm/distance-hashing"
)

func main() {
	// Create a canonical session generator (recommended for production)
	csg, err := dh.NewCanonicalSessionGenerator(10000)
	if err != nil {
		log.Fatalf("Failed to create canonical session generator: %v", err)
	}

	// Demonstrate priority-based canonical selection
	fmt.Println("=== Priority-Based Session Keys ===")
	fmt.Println("Priority: UserID > Email > ClientID > DeviceID > CookieID > JwtToken")

	// Step 1: Start with low-priority identifier (cookie)
	fmt.Println("\n1. Cookie only:")
	key1 := csg.GetSessionKey(dh.Identifiers{
		dh.IdentifierCookie: "cookie_abc",
	})
	fmt.Printf("   Session key: %s\n", key1)

	// Step 2: Add email (higher priority than cookie)
	fmt.Println("\n2. Link email (higher priority):")
	csg.LinkIdentifiers("cookie:cookie_abc", "email:user@example.com")
	key2 := csg.GetSessionKey(dh.Identifiers{
		dh.IdentifierCookie: "cookie_abc",
	})
	fmt.Printf("   Session key: %s (changed due to higher priority)\n", key2)

	// Step 3: Add user ID (highest priority)
	fmt.Println("\n3. Link user ID (highest priority):")
	csg.LinkIdentifiers("email:user@example.com", "uid:user_123")
	key3 := csg.GetSessionKey(dh.Identifiers{
		dh.IdentifierCookie: "cookie_abc",
	})
	fmt.Printf("   Session key: %s (changed to uid-based)\n", key3)

	// Step 4: Add JWT token (lower priority - key should NOT change)
	fmt.Println("\n4. Link JWT token (lower priority):")
	csg.LinkIdentifiers("uid:user_123", "jwt:token_xyz")
	key4 := csg.GetSessionKey(dh.Identifiers{
		dh.IdentifierCookie: "cookie_abc",
	})
	fmt.Printf("   Session key: %s (stable - uid still canonical)\n", key4)

	// Verify stability
	fmt.Println("\n=== Stability Verification ===")
	fmt.Printf("Key after uid link: %s\n", key3)
	fmt.Printf("Key after jwt link: %s\n", key4)
	fmt.Printf("Keys are stable: %v ✅\n", key3 == key4)

	// All identifiers return the same final key
	fmt.Println("\n=== All Identifiers Return Same Key ===")
	keyUser := csg.GetSessionKey(dh.Identifiers{dh.IdentifierUserID: "user_123"})
	keyEmail := csg.GetSessionKey(dh.Identifiers{dh.IdentifierEmail: "user@example.com"})
	keyJWT := csg.GetSessionKey(dh.Identifiers{dh.IdentifierJWT: "token_xyz"})
	keyCookie := csg.GetSessionKey(dh.Identifiers{dh.IdentifierCookie: "cookie_abc"})

	fmt.Printf("dh.IdentifierUserID:   %s\n", keyUser)
	fmt.Printf("dh.IdentifierEmail:    %s\n", keyEmail)
	fmt.Printf("JWT:      %s\n", keyJWT)
	fmt.Printf("Cookie:   %s\n", keyCookie)
	fmt.Printf("All same: %v ✅\n", keyUser == keyEmail && keyEmail == keyJWT && keyJWT == keyCookie)

	// Session size
	size := csg.GetSessionSize("uid:user_123")
	fmt.Printf("\nSession size: %d identifiers\n", size)

	// Statistics
	stats := csg.GetStats()
	fmt.Printf("\n=== Statistics ===\n")
	fmt.Printf("Total identifiers: %d\n", stats.TotalIdentifiers)
	fmt.Printf("Total sessions: %d\n", stats.TotalSessions)
	fmt.Printf("Cache size: %d\n", stats.CacheSize)
}

package main

import (
	"fmt"
	"log"

	dh "github.com/wallarm/distance-hashing"
)

func main() {
	// Create a session generator with 10K cache size
	sg, err := dh.NewSessionGenerator(10000)
	if err != nil {
		log.Fatalf("Failed to create session generator: %v", err)
	}

	// Scenario 1: Anonymous user with cookie
	fmt.Println("=== Scenario 1: Anonymous User ===")
	sessionKey1 := sg.GetSessionKey(dh.Identifiers{
		dh.IdentifierCookie: "cookie_abc123",
	})
	fmt.Printf("Anonymous session key: %s\n", sessionKey1)

	// Scenario 2: User logs in - link cookie to user ID
	fmt.Println("\n=== Scenario 2: User Logs In ===")
	sg.LinkIdentifiers("cookie:cookie_abc123", "uid:user_42")
	sessionKey2 := sg.GetSessionKey(dh.Identifiers{
		dh.IdentifierCookie: "cookie_abc123",
	})
	fmt.Printf("Session key after login: %s\n", sessionKey2)

	// Scenario 3: JWT token issued
	fmt.Println("\n=== Scenario 3: JWT Token Issued ===")
	sg.LinkIdentifiers("uid:user_42", "jwt:jwt_token_xyz")
	sessionKey3 := sg.GetSessionKey(dh.Identifiers{
		dh.IdentifierJWT: "jwt_token_xyz",
	})
	fmt.Printf("Session key with JWT: %s\n", sessionKey3)

	// Verify all identifiers return the SAME session key
	fmt.Println("\n=== Verification ===")
	key1 := sg.GetSessionKey(dh.Identifiers{dh.IdentifierCookie: "cookie_abc123"})
	key2 := sg.GetSessionKey(dh.Identifiers{dh.IdentifierUserID: "user_42"})
	key3 := sg.GetSessionKey(dh.Identifiers{dh.IdentifierJWT: "jwt_token_xyz"})

	fmt.Printf("Cookie session key:  %s\n", key1)
	fmt.Printf("User ID session key: %s\n", key2)
	fmt.Printf("JWT session key:     %s\n", key3)
	fmt.Printf("All identical: %v\n", key1 == key2 && key2 == key3)

	// Get session size
	size := sg.GetSessionSize("uid:user_42")
	fmt.Printf("\nSession size (number of identifiers): %d\n", size)

	// Get statistics
	stats := sg.GetStats()
	fmt.Printf("\n=== Statistics ===\n")
	fmt.Printf("Total identifiers: %d\n", stats.TotalIdentifiers)
	fmt.Printf("Total sessions: %d\n", stats.TotalSessions)
	fmt.Printf("Cache size: %d\n", stats.CacheSize)
}

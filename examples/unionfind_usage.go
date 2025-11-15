package main

import (
	"fmt"

	dh "github.com/wallarm/distance-hashing"
)

func main() {
	// Create a Union-Find instance
	uf := dh.NewUnionFind()

	fmt.Println("=== Union-Find Example ===")
	fmt.Println("\nScenario: Track user session evolution")

	// Step 1: Anonymous user with cookie
	fmt.Println("\n1. Anonymous user visits site:")
	cookie1 := "cookie:session_abc"
	root1 := uf.Find(cookie1)
	fmt.Printf("   Cookie: %s -> Root: %s\n", cookie1, root1)

	// Step 2: User logs in - union cookie with user ID
	fmt.Println("\n2. User logs in:")
	userID := "uid:user_123"
	root2 := uf.Union(cookie1, userID)
	fmt.Printf("   Union(%s, %s) -> Root: %s\n", cookie1, userID, root2)

	// Verify connectivity
	if uf.Connected(cookie1, userID) {
		fmt.Printf("   ✅ Cookie and UserID are connected\n")
	}

	// Step 3: JWT token issued
	fmt.Println("\n3. JWT token issued:")
	jwtToken := "jwt:token_xyz"
	root3 := uf.Union(userID, jwtToken)
	fmt.Printf("   Union(%s, %s) -> Root: %s\n", userID, jwtToken, root3)

	// Step 4: User from different device
	fmt.Println("\n4. Same user from different device:")
	cookie2 := "cookie:session_def"
	root4 := uf.Union(jwtToken, cookie2)
	fmt.Printf("   Union(%s, %s) -> Root: %s\n", jwtToken, cookie2, root4)

	// Verify all are connected
	fmt.Println("\n=== Connectivity Verification ===")
	identifiers := []string{cookie1, userID, jwtToken, cookie2}
	for i, id1 := range identifiers {
		for j, id2 := range identifiers {
			if i < j {
				connected := uf.Connected(id1, id2)
				fmt.Printf("%s <-> %s: %v\n", id1, id2, connected)
			}
		}
	}

	// Get component size
	size := uf.ComponentSize(userID)
	fmt.Printf("\nComponent size for %s: %d identifiers\n", userID, size)

	// Show all components
	fmt.Println("\n=== All Components ===")
	components := uf.GetAllComponents()
	for root, members := range components {
		fmt.Printf("Root: %s\n", root)
		for _, member := range members {
			fmt.Printf("  - %s\n", member)
		}
	}

	// Demonstrate fraud detection use case
	fmt.Println("\n=== Fraud Detection Use Case ===")
	fmt.Println("Adding suspicious activity patterns...")

	// Account 1
	uf.Union("uid:user_999", "cookie:suspicious_1")
	uf.Union("uid:user_999", "device:phone_aaa")

	// Account 2 (shares device with account 1)
	uf.Union("uid:user_888", "cookie:suspicious_2")
	uf.Union("uid:user_888", "device:phone_aaa") // Same device!

	// Now user_999 and user_888 are connected through shared device
	if uf.Connected("uid:user_999", "uid:user_888") {
		fmt.Println("⚠️  ALERT: user_999 and user_888 share same device!")
		fmt.Println("    This could indicate:")
		fmt.Println("    - Same person with multiple accounts")
		fmt.Println("    - Account takeover")
		fmt.Println("    - Fraud ring")

		// Get all identifiers in this suspicious network
		suspiciousSize := uf.ComponentSize("uid:user_999")
		fmt.Printf("    Suspicious network size: %d identifiers\n", suspiciousSize)
	}

	// Final statistics
	fmt.Println("\n=== Final Statistics ===")
	allComponents := uf.GetAllComponents()
	totalIdentifiers := uf.Size()
	fmt.Printf("Total identifiers: %d\n", totalIdentifiers)
	fmt.Printf("Total components: %d\n", len(allComponents))
	fmt.Printf("Average component size: %.1f\n", float64(totalIdentifiers)/float64(len(allComponents)))
}

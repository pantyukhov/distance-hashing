package distancehashing

import (
	"fmt"
	"sync"
	"testing"
)

func TestUnionFind_BasicOperations(t *testing.T) {
	uf := NewUnionFind()

	// Test 1: Find on new element creates it
	root1 := uf.Find("user_1")
	if root1 != "user_1" {
		t.Errorf("Expected root to be 'user_1', got '%s'", root1)
	}

	// Test 2: Find on same element returns same root
	root2 := uf.Find("user_1")
	if root2 != root1 {
		t.Errorf("Expected consistent root, got '%s' vs '%s'", root1, root2)
	}

	// Test 3: Two separate elements have different roots
	root3 := uf.Find("user_2")
	if root3 == root1 {
		t.Error("Different users should have different roots")
	}

	// Test 4: Connected returns false for unconnected elements
	if uf.Connected("user_1", "user_2") {
		t.Error("user_1 and user_2 should not be connected")
	}
}

func TestUnionFind_Union(t *testing.T) {
	uf := NewUnionFind()

	// Create three separate elements
	uf.Find("jwt_1")
	uf.Find("jwt_2")
	uf.Find("user_id")

	// Union jwt_1 and user_id
	root := uf.Union("jwt_1", "user_id")

	// They should now be connected
	if !uf.Connected("jwt_1", "user_id") {
		t.Error("jwt_1 and user_id should be connected after Union")
	}

	// Their roots should be the same
	if uf.Find("jwt_1") != uf.Find("user_id") {
		t.Error("jwt_1 and user_id should have the same root")
	}

	// jwt_2 should still be separate
	if uf.Connected("jwt_1", "jwt_2") {
		t.Error("jwt_1 and jwt_2 should not be connected")
	}

	// Union jwt_2 with user_id
	uf.Union("jwt_2", "user_id")

	// Now all three should be connected (transitivity test)
	if !uf.Connected("jwt_1", "jwt_2") {
		t.Error("jwt_1 and jwt_2 should be connected through user_id (transitivity)")
	}

	// All should have the same root
	root1 := uf.Find("jwt_1")
	root2 := uf.Find("jwt_2")
	rootUser := uf.Find("user_id")

	if root1 != root2 || root2 != rootUser {
		t.Errorf("All elements should have same root: %s, %s, %s", root1, root2, rootUser)
	}

	// The returned root from Union should match
	if root != root1 {
		t.Errorf("Union return value should match Find result: %s vs %s", root, root1)
	}
}

func TestUnionFind_Transitivity(t *testing.T) {
	uf := NewUnionFind()

	// Create a chain: A -> B -> C -> D -> E
	// This simulates the diagram from the spec:
	// Session XXX -> JWT1 -> User -> JWT2 -> Session YYY
	chain := []string{"session_xxx", "jwt_1", "user_12345", "jwt_2", "session_yyy"}

	// Link them sequentially
	for i := 0; i < len(chain)-1; i++ {
		uf.Union(chain[i], chain[i+1])
	}

	// All elements should be connected
	root := uf.Find(chain[0])
	for _, id := range chain {
		if uf.Find(id) != root {
			t.Errorf("Element %s should have root %s, got %s", id, root, uf.Find(id))
		}
	}

	// First and last should be connected (transitivity)
	if !uf.Connected("session_xxx", "session_yyy") {
		t.Error("session_xxx and session_yyy should be connected through the chain")
	}

	// Component size should be 5
	size := uf.ComponentSize(chain[0])
	if size != 5 {
		t.Errorf("Component size should be 5, got %d", size)
	}
}

func TestUnionFind_MultipleComponents(t *testing.T) {
	uf := NewUnionFind()

	// Create two separate components
	// Component 1: user_1, jwt_1, session_1
	uf.Union("user_1", "jwt_1")
	uf.Union("jwt_1", "session_1")

	// Component 2: user_2, jwt_2, session_2
	uf.Union("user_2", "jwt_2")
	uf.Union("jwt_2", "session_2")

	// Elements within same component should be connected
	if !uf.Connected("user_1", "session_1") {
		t.Error("user_1 and session_1 should be connected")
	}
	if !uf.Connected("user_2", "session_2") {
		t.Error("user_2 and session_2 should be connected")
	}

	// Elements from different components should NOT be connected
	if uf.Connected("user_1", "user_2") {
		t.Error("user_1 and user_2 should not be connected")
	}

	// Check component sizes
	size1 := uf.ComponentSize("user_1")
	size2 := uf.ComponentSize("user_2")

	if size1 != 3 {
		t.Errorf("Component 1 size should be 3, got %d", size1)
	}
	if size2 != 3 {
		t.Errorf("Component 2 size should be 3, got %d", size2)
	}

	// Total size should be 6
	if uf.Size() != 6 {
		t.Errorf("Total size should be 6, got %d", uf.Size())
	}

	// Get all components
	components := uf.GetAllComponents()
	if len(components) != 2 {
		t.Errorf("Should have 2 components, got %d", len(components))
	}
}

func TestUnionFind_ConcurrentAccess(t *testing.T) {
	uf := NewUnionFind()
	const numGoroutines = 100
	const operationsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch multiple goroutines performing concurrent operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			base := fmt.Sprintf("user_%d", id)

			for j := 0; j < operationsPerGoroutine; j++ {
				nodeID := fmt.Sprintf("%s_op_%d", base, j)

				// Mix of Find and Union operations
				if j%2 == 0 {
					uf.Find(nodeID)
				} else {
					uf.Union(base, nodeID)
				}

				// Random connectivity checks
				if j%3 == 0 {
					uf.Connected(base, nodeID)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify no panics occurred and data structure is consistent
	size := uf.Size()
	if size == 0 {
		t.Error("UnionFind should have elements after concurrent operations")
	}

	t.Logf("Successfully completed %d concurrent operations, total size: %d",
		numGoroutines*operationsPerGoroutine, size)
}

func TestUnionFind_IdempotentUnion(t *testing.T) {
	uf := NewUnionFind()

	// Union same elements multiple times
	root1 := uf.Union("a", "b")
	root2 := uf.Union("a", "b")
	root3 := uf.Union("b", "a")

	// All should return the same root
	if root1 != root2 || root2 != root3 {
		t.Errorf("Idempotent unions should return same root: %s, %s, %s", root1, root2, root3)
	}

	// Component size should still be 2
	size := uf.ComponentSize("a")
	if size != 2 {
		t.Errorf("Component size should be 2, got %d", size)
	}
}

func TestUnionFind_Clear(t *testing.T) {
	uf := NewUnionFind()

	// Add some elements
	uf.Union("a", "b")
	uf.Union("c", "d")

	if uf.Size() == 0 {
		t.Error("Should have elements before Clear")
	}

	// Clear the structure
	uf.Clear()

	if uf.Size() != 0 {
		t.Errorf("Size should be 0 after Clear, got %d", uf.Size())
	}

	// Should be able to add elements again
	uf.Union("x", "y")
	if uf.Size() != 2 {
		t.Errorf("Size should be 2 after adding new elements, got %d", uf.Size())
	}
}

func TestUnionFind_PathCompression(t *testing.T) {
	uf := NewUnionFind()

	// Create a long chain without path compression would be slow
	// a -> b -> c -> d -> e -> f -> g -> h -> i -> j
	chain := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}

	for i := 0; i < len(chain)-1; i++ {
		uf.Union(chain[i], chain[i+1])
	}

	// First Find() will compress the path
	root := uf.Find(chain[0])

	// After path compression, all nodes should point directly to root
	// Subsequent Find() operations should be O(1)
	for _, node := range chain {
		if uf.Find(node) != root {
			t.Errorf("Node %s should have root %s", node, root)
		}
	}

	// All should be connected
	if !uf.Connected(chain[0], chain[len(chain)-1]) {
		t.Error("First and last elements should be connected")
	}
}

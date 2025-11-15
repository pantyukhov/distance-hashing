package distancehashing

import (
	"sync"
)

// UnionFind implements the Disjoint Set Union (DSU) data structure
// with path compression and union by rank optimizations.
// This provides near-constant time O(α(n)) operations where α is the inverse Ackermann function.
//
// Thread-safe for concurrent operations.
type UnionFind struct {
	parent map[string]string // parent[x] = parent of x in the tree
	rank   map[string]int    // rank[x] = approximate depth of tree rooted at x
	mu     sync.RWMutex      // protects concurrent access
}

// NewUnionFind creates a new UnionFind data structure.
func NewUnionFind() *UnionFind {
	return &UnionFind{
		parent: make(map[string]string),
		rank:   make(map[string]int),
	}
}

// Find returns the representative (root) of the set containing id.
// Applies path compression optimization: all nodes on the path point directly to root.
//
// Time complexity: O(α(n)) amortized, where α is the inverse Ackermann function
// (practically constant time - α(n) < 5 for any realistic n)
func (uf *UnionFind) Find(id string) string {
	uf.mu.Lock()
	defer uf.mu.Unlock()

	return uf.findWithoutLock(id)
}

// findWithoutLock is the internal Find implementation without locking.
// Used by other methods that already hold the lock.
func (uf *UnionFind) findWithoutLock(id string) string {
	// If id doesn't exist, create new set with id as root
	if _, exists := uf.parent[id]; !exists {
		uf.parent[id] = id
		uf.rank[id] = 0
		return id
	}

	// Path compression: make every node point directly to root
	if uf.parent[id] != id {
		uf.parent[id] = uf.findWithoutLock(uf.parent[id])
	}

	return uf.parent[id]
}

// Union merges the sets containing id1 and id2.
// Returns the representative of the merged set.
//
// Uses union by rank optimization: attach smaller tree under root of larger tree.
// This keeps the tree relatively flat, ensuring O(α(n)) time complexity.
//
// Time complexity: O(α(n)) amortized
func (uf *UnionFind) Union(id1, id2 string) string {
	uf.mu.Lock()
	defer uf.mu.Unlock()

	root1 := uf.findWithoutLock(id1)
	root2 := uf.findWithoutLock(id2)

	// Already in the same set
	if root1 == root2 {
		return root1
	}

	// Union by rank: attach smaller tree under root of larger tree
	if uf.rank[root1] < uf.rank[root2] {
		uf.parent[root1] = root2
		return root2
	} else if uf.rank[root1] > uf.rank[root2] {
		uf.parent[root2] = root1
		return root1
	} else {
		// Equal rank: choose root1 as parent and increase its rank
		uf.parent[root2] = root1
		uf.rank[root1]++
		return root1
	}
}

// Connected returns true if id1 and id2 are in the same set (same session).
//
// Time complexity: O(α(n)) amortized
func (uf *UnionFind) Connected(id1, id2 string) bool {
	return uf.Find(id1) == uf.Find(id2)
}

// ComponentSize returns the number of elements in the set containing id.
//
// Time complexity: O(n) where n is total number of elements
// Note: This is an expensive operation. Use sparingly.
func (uf *UnionFind) ComponentSize(id string) int {
	root := uf.Find(id)

	uf.mu.RLock()
	defer uf.mu.RUnlock()

	size := 0
	for nodeID := range uf.parent {
		if uf.findWithoutLock(nodeID) == root {
			size++
		}
	}

	return size
}

// Size returns the total number of elements tracked by this UnionFind.
func (uf *UnionFind) Size() int {
	uf.mu.RLock()
	defer uf.mu.RUnlock()
	return len(uf.parent)
}

// GetAllComponents returns a map of root -> list of all members in that component.
// This is useful for debugging and state snapshots.
//
// Time complexity: O(n) where n is total number of elements
func (uf *UnionFind) GetAllComponents() map[string][]string {
	uf.mu.Lock()
	defer uf.mu.Unlock()

	components := make(map[string][]string)

	for nodeID := range uf.parent {
		root := uf.findWithoutLock(nodeID)
		components[root] = append(components[root], nodeID)
	}

	return components
}

// GetComponentMembers returns all members of the component containing the given ID.
// This is an atomic operation that avoids race conditions.
//
// Time complexity: O(n) where n is total number of elements
func (uf *UnionFind) GetComponentMembers(id string) []string {
	uf.mu.Lock()
	defer uf.mu.Unlock()

	root := uf.findWithoutLock(id)
	var members []string

	for nodeID := range uf.parent {
		if uf.findWithoutLock(nodeID) == root {
			members = append(members, nodeID)
		}
	}

	return members
}

// Clear removes all elements from the UnionFind structure.
// Useful for testing or periodic cleanup.
func (uf *UnionFind) Clear() {
	uf.mu.Lock()
	defer uf.mu.Unlock()

	uf.parent = make(map[string]string)
	uf.rank = make(map[string]int)
}

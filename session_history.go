package distancehashing

import (
	"sync"
	"time"
)

// SessionKeyHistory tracks the evolution of session keys as the identity graph grows.
// This solves the temporal problem where session keys change over time as new identifiers are linked.
//
// Example:
//   10:00 - Anonymous visit → session_key = "sess_ABC"
//   10:30 - User logs in → session_key = "sess_XYZ" (changed!)
//
// With history tracking, you can query all events for both "sess_ABC" and "sess_XYZ"
// to get the complete user journey.
type SessionKeyHistory struct {
	CurrentKey string    // Current active session key
	OldKeys    []string  // All previous session keys (chronologically)
	UpdatedAt  time.Time // Last update timestamp
}

// SessionGeneratorWithHistory wraps SessionGenerator and tracks session key changes over time.
// This is the production-ready solution for handling session key instability.
type SessionGeneratorWithHistory struct {
	*SessionGenerator

	// Maps current session key → history of old keys
	history map[string]*SessionKeyHistory

	// Reverse index: old key → current key (for quick lookups)
	oldToNew map[string]string

	mu sync.RWMutex
}

// NewSessionGeneratorWithHistory creates a new generator that tracks session key history.
func NewSessionGeneratorWithHistory(cacheSize int) (*SessionGeneratorWithHistory, error) {
	sg, err := NewSessionGenerator(cacheSize)
	if err != nil {
		return nil, err
	}

	return &SessionGeneratorWithHistory{
		SessionGenerator: sg,
		history:          make(map[string]*SessionKeyHistory),
		oldToNew:         make(map[string]string),
	}, nil
}

// GetSessionKey returns the current session key and tracks history if it changes.
func (sgh *SessionGeneratorWithHistory) GetSessionKey(ids Identifiers) string {
	// Get any identifier from the set to check for previous key
	var sampleID string
	for idType, idValue := range ids {
		if idValue != "" {
			sampleID = idType + ":" + idValue
			break
		}
	}

	// Check what the OLD key was before this call
	var oldKey string
	if sampleID != "" {
		sgh.SessionGenerator.mu.RLock()
		if cached, ok := sgh.SessionGenerator.cache.Get(sampleID); ok {
			oldKey = cached
		}
		sgh.SessionGenerator.mu.RUnlock()
	}

	// Get current key (may create new links and change the key)
	newKey := sgh.SessionGenerator.GetSessionKey(ids)

	// Track history if key changed
	if oldKey != "" && oldKey != newKey {
		sgh.trackKeyChange(oldKey, newKey)
	} else if oldKey == "" {
		// First time seeing this session - initialize history
		sgh.initializeHistory(newKey)
	}

	return newKey
}

// LinkIdentifiers links two identifiers and tracks any session key changes.
func (sgh *SessionGeneratorWithHistory) LinkIdentifiers(id1, id2 string) {
	if id1 == "" || id2 == "" {
		return
	}

	// Get old keys BEFORE linking
	sgh.SessionGenerator.mu.Lock()

	// Check cache first
	oldKey1, hasOld1 := sgh.SessionGenerator.cache.Get(id1)
	if !hasOld1 {
		component := sgh.SessionGenerator.findConnectedComponentWithoutLock(id1)
		oldKey1 = sgh.SessionGenerator.computeComponentCanonicalHash(component)
	}

	oldKey2, hasOld2 := sgh.SessionGenerator.cache.Get(id2)
	if !hasOld2 {
		component := sgh.SessionGenerator.findConnectedComponentWithoutLock(id2)
		oldKey2 = sgh.SessionGenerator.computeComponentCanonicalHash(component)
	}

	// Add edge and invalidate caches
	sgh.SessionGenerator.addEdgeWithoutLock(id1, id2)
	sgh.SessionGenerator.cache.Remove(id1)
	sgh.SessionGenerator.cache.Remove(id2)

	// Invalidate hash cache for the affected component
	component := sgh.SessionGenerator.findConnectedComponentWithoutLock(id1)
	for nodeID := range component {
		delete(sgh.SessionGenerator.hashCache, nodeID)
	}

	// Compute new key after linking
	newKey := sgh.SessionGenerator.computeComponentCanonicalHash(component)

	sgh.SessionGenerator.mu.Unlock()

	// Track history for any keys that changed
	if oldKey1 != newKey {
		sgh.trackKeyChange(oldKey1, newKey)
	}
	if oldKey2 != newKey && oldKey2 != oldKey1 {
		sgh.trackKeyChange(oldKey2, newKey)
	}
}

// GetSessionKeyHistory returns the full history for a session key (current or old).
// This allows you to query all events across all historical keys.
func (sgh *SessionGeneratorWithHistory) GetSessionKeyHistory(sessionKey string) *SessionKeyHistory {
	sgh.mu.RLock()
	defer sgh.mu.RUnlock()

	// Check if this is an old key - map to current
	if currentKey, isOld := sgh.oldToNew[sessionKey]; isOld {
		sessionKey = currentKey
	}

	// Return history for current key
	if history, ok := sgh.history[sessionKey]; ok {
		// Return a copy to prevent external modifications
		return &SessionKeyHistory{
			CurrentKey: history.CurrentKey,
			OldKeys:    append([]string{}, history.OldKeys...),
			UpdatedAt:  history.UpdatedAt,
		}
	}

	// No history found - this is a new session
	return &SessionKeyHistory{
		CurrentKey: sessionKey,
		OldKeys:    []string{},
		UpdatedAt:  time.Now(),
	}
}

// GetAllSessionKeys returns both current and all historical session keys.
// Use this when querying analytics/events to get the complete user journey.
//
// Example:
//   allKeys := sgh.GetAllSessionKeys(currentSessionKey)
//   events := db.Query("SELECT * FROM events WHERE session_key IN (?)", allKeys)
func (sgh *SessionGeneratorWithHistory) GetAllSessionKeys(sessionKey string) []string {
	history := sgh.GetSessionKeyHistory(sessionKey)

	allKeys := []string{history.CurrentKey}
	allKeys = append(allKeys, history.OldKeys...)

	return allKeys
}

// trackKeyChange records that a session key has changed from oldKey to newKey.
func (sgh *SessionGeneratorWithHistory) trackKeyChange(oldKey, newKey string) {
	if oldKey == newKey {
		return
	}

	sgh.mu.Lock()
	defer sgh.mu.Unlock()

	now := time.Now()

	// Get or create history for new key
	newHistory, exists := sgh.history[newKey]
	if !exists {
		newHistory = &SessionKeyHistory{
			CurrentKey: newKey,
			OldKeys:    []string{},
			UpdatedAt:  now,
		}
		sgh.history[newKey] = newHistory
	}

	// Add old key to history if not already present
	alreadyTracked := false
	for _, k := range newHistory.OldKeys {
		if k == oldKey {
			alreadyTracked = true
			break
		}
	}

	if !alreadyTracked {
		newHistory.OldKeys = append(newHistory.OldKeys, oldKey)
		newHistory.UpdatedAt = now
	}

	// Update reverse index
	sgh.oldToNew[oldKey] = newKey

	// If oldKey had its own history, merge it
	if oldHistory, hadHistory := sgh.history[oldKey]; hadHistory {
		// Merge old history into new
		for _, ancestorKey := range oldHistory.OldKeys {
			// Avoid duplicates
			isDuplicate := false
			for _, k := range newHistory.OldKeys {
				if k == ancestorKey {
					isDuplicate = true
					break
				}
			}
			if !isDuplicate {
				newHistory.OldKeys = append(newHistory.OldKeys, ancestorKey)
			}

			// Update reverse index for ancestors
			sgh.oldToNew[ancestorKey] = newKey
		}

		// Remove old history entry (it's been merged)
		delete(sgh.history, oldKey)
	}
}

// initializeHistory creates initial history entry for a new session.
func (sgh *SessionGeneratorWithHistory) initializeHistory(sessionKey string) {
	sgh.mu.Lock()
	defer sgh.mu.Unlock()

	if _, exists := sgh.history[sessionKey]; !exists {
		sgh.history[sessionKey] = &SessionKeyHistory{
			CurrentKey: sessionKey,
			OldKeys:    []string{},
			UpdatedAt:  time.Now(),
		}
	}
}

// GetStats returns statistics including history tracking info.
type StatsWithHistory struct {
	Stats                   // Embedded base stats
	TotalHistoricalKeys int // Total number of historical keys tracked
	SessionsWithHistory int // Sessions that have experienced key changes
}

// GetStatsWithHistory returns statistics including history information.
func (sgh *SessionGeneratorWithHistory) GetStatsWithHistory() StatsWithHistory {
	baseStats := sgh.SessionGenerator.GetStats()

	sgh.mu.RLock()
	defer sgh.mu.RUnlock()

	totalHistorical := len(sgh.oldToNew)
	sessionsWithHistory := 0
	for _, history := range sgh.history {
		if len(history.OldKeys) > 0 {
			sessionsWithHistory++
		}
	}

	return StatsWithHistory{
		Stats:               baseStats,
		TotalHistoricalKeys: totalHistorical,
		SessionsWithHistory: sessionsWithHistory,
	}
}

// Clear removes all history and resets the generator.
func (sgh *SessionGeneratorWithHistory) Clear() {
	sgh.SessionGenerator.Clear()

	sgh.mu.Lock()
	defer sgh.mu.Unlock()

	sgh.history = make(map[string]*SessionKeyHistory)
	sgh.oldToNew = make(map[string]string)
}

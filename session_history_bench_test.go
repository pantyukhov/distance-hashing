package distancehashing

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// BenchmarkHistoryOverhead measures overhead of history tracking
func BenchmarkHistoryOverhead(b *testing.B) {
	b.Run("WithoutHistory_GetSessionKey", func(b *testing.B) {
		sg, _ := NewSessionGenerator(10000)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sg.GetSessionKey(Identifiers{
				IdentifierUserID: "user_42",
				IdentifierCookie: "abc123",
			})
		}
	})

	b.Run("WithHistory_GetSessionKey", func(b *testing.B) {
		sgh, _ := NewSessionGeneratorWithHistory(10000)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sgh.GetSessionKey(Identifiers{
				IdentifierUserID: "user_42",
				IdentifierCookie: "abc123",
			})
		}
	})

	b.Run("WithoutHistory_LinkIdentifiers", func(b *testing.B) {
		sg, _ := NewSessionGenerator(10000)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			id := fmt.Sprintf("user_%d", i)
			sg.LinkIdentifiers("cookie:abc", "uid:"+id)
		}
	})

	b.Run("WithHistory_LinkIdentifiers", func(b *testing.B) {
		sgh, _ := NewSessionGeneratorWithHistory(10000)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			id := fmt.Sprintf("user_%d", i)
			sgh.LinkIdentifiers("cookie:abc", "uid:"+id)
		}
	})

	b.Run("WithHistory_GetAllSessionKeys", func(b *testing.B) {
		sgh, _ := NewSessionGeneratorWithHistory(10000)
		// Build up some history
		sgh.GetSessionKey(Identifiers{IdentifierCookie: "abc"})
		sgh.LinkIdentifiers("cookie:abc", "uid:42")
		sgh.LinkIdentifiers("uid:42", "email:test@example.com")
		sgh.LinkIdentifiers("email:test@example.com", "device:mobile")
		key := sgh.GetSessionKey(Identifiers{IdentifierDevice: "mobile"})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sgh.GetAllSessionKeys(key)
		}
	})
}

// BenchmarkHighThroughput simulates production load
func BenchmarkHighThroughput(b *testing.B) {
	b.Run("Without_History_100K_RPS", func(b *testing.B) {
		sg, _ := NewSessionGenerator(10000)
		b.SetParallelism(100) // 100 goroutines
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				userID := fmt.Sprintf("user_%d", i%1000)
				sg.GetSessionKey(Identifiers{
					IdentifierUserID: userID,
					IdentifierCookie: "cookie_" + userID,
				})
				i++
			}
		})
	})

	b.Run("With_History_100K_RPS", func(b *testing.B) {
		sgh, _ := NewSessionGeneratorWithHistory(10000)
		b.SetParallelism(100)
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				userID := fmt.Sprintf("user_%d", i%1000)
				sgh.GetSessionKey(Identifiers{
					IdentifierUserID: userID,
					IdentifierCookie: "cookie_" + userID,
				})
				i++
			}
		})
	})
}

// TestMemoryGrowth tests memory usage with many sessions
func TestMemoryGrowth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	t.Run("1M_Sessions_Without_History", func(t *testing.T) {
		sg, _ := NewSessionGenerator(10000)

		start := time.Now()
		for i := 0; i < 1_000_000; i++ {
			sg.GetSessionKey(Identifiers{
				IdentifierUserID: fmt.Sprintf("user_%d", i),
			})
		}
		elapsed := time.Since(start)

		stats := sg.GetStats()
		t.Logf("1M sessions WITHOUT history:")
		t.Logf("  Time: %v", elapsed)
		t.Logf("  Avg: %.2f µs/session", float64(elapsed.Microseconds())/1_000_000)
		t.Logf("  Identifiers: %d", stats.TotalIdentifiers)
		t.Logf("  Sessions: %d", stats.TotalSessions)
	})

	t.Run("1M_Sessions_With_History", func(t *testing.T) {
		sgh, _ := NewSessionGeneratorWithHistory(10000)

		start := time.Now()
		for i := 0; i < 1_000_000; i++ {
			sgh.GetSessionKey(Identifiers{
				IdentifierUserID: fmt.Sprintf("user_%d", i),
			})
		}
		elapsed := time.Since(start)

		stats := sgh.GetStatsWithHistory()
		t.Logf("1M sessions WITH history:")
		t.Logf("  Time: %v", elapsed)
		t.Logf("  Avg: %.2f µs/session", float64(elapsed.Microseconds())/1_000_000)
		t.Logf("  Identifiers: %d", stats.TotalIdentifiers)
		t.Logf("  Sessions: %d", stats.TotalSessions)
		t.Logf("  Historical keys: %d", stats.TotalHistoricalKeys)
	})

	t.Run("1M_Sessions_With_Links_And_History", func(t *testing.T) {
		sgh, _ := NewSessionGeneratorWithHistory(10000)

		start := time.Now()
		for i := 0; i < 1_000_000; i++ {
			// Simulate user journey: anonymous → login
			cookie := fmt.Sprintf("cookie_%d", i)
			userID := fmt.Sprintf("user_%d", i)

			// Anonymous
			sgh.GetSessionKey(Identifiers{IdentifierCookie: cookie})

			// Login - creates history
			sgh.LinkIdentifiers("cookie:"+cookie, "uid:"+userID)
			sgh.GetSessionKey(Identifiers{IdentifierUserID: userID})
		}
		elapsed := time.Since(start)

		stats := sgh.GetStatsWithHistory()
		t.Logf("1M sessions WITH links and history:")
		t.Logf("  Time: %v", elapsed)
		t.Logf("  Avg: %.2f µs/session", float64(elapsed.Microseconds())/1_000_000)
		t.Logf("  Identifiers: %d", stats.TotalIdentifiers)
		t.Logf("  Sessions: %d", stats.TotalSessions)
		t.Logf("  Historical keys: %d", stats.TotalHistoricalKeys)
		t.Logf("  Sessions with history: %d", stats.SessionsWithHistory)
		t.Logf("  History ratio: %.2f%%", float64(stats.SessionsWithHistory)/float64(stats.TotalSessions)*100)
	})
}

// TestConcurrentHistoryAccess tests thread safety under load
func TestConcurrentHistoryAccess(t *testing.T) {
	sgh, _ := NewSessionGeneratorWithHistory(10000)

	const numGoroutines = 100
	const opsPerGoroutine = 1000

	// Build initial graph
	for i := 0; i < 10; i++ {
		sgh.GetSessionKey(Identifiers{IdentifierUserID: fmt.Sprintf("user_%d", i)})
	}

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	start := time.Now()

	// Spawn goroutines doing random operations
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()

			for i := 0; i < opsPerGoroutine; i++ {
				op := i % 4

				switch op {
				case 0: // GetSessionKey
					userID := fmt.Sprintf("user_%d", (goroutineID+i)%100)
					sgh.GetSessionKey(Identifiers{IdentifierUserID: userID})

				case 1: // LinkIdentifiers
					id1 := fmt.Sprintf("user_%d", (goroutineID+i)%100)
					id2 := fmt.Sprintf("email_%d", (goroutineID+i)%100)
					sgh.LinkIdentifiers("uid:"+id1, "email:"+id2)

				case 2: // GetAllSessionKeys
					userID := fmt.Sprintf("user_%d", (goroutineID+i)%100)
					key := sgh.GetSessionKey(Identifiers{IdentifierUserID: userID})
					sgh.GetAllSessionKeys(key)

				case 3: // GetSessionKeyHistory
					userID := fmt.Sprintf("user_%d", (goroutineID+i)%100)
					key := sgh.GetSessionKey(Identifiers{IdentifierUserID: userID})
					sgh.GetSessionKeyHistory(key)
				}
			}
		}(g)
	}

	wg.Wait()
	elapsed := time.Since(start)

	totalOps := numGoroutines * opsPerGoroutine
	opsPerSec := float64(totalOps) / elapsed.Seconds()

	t.Logf("\n=== Concurrent Access Test ===")
	t.Logf("Goroutines: %d", numGoroutines)
	t.Logf("Ops per goroutine: %d", opsPerGoroutine)
	t.Logf("Total ops: %d", totalOps)
	t.Logf("Time: %v", elapsed)
	t.Logf("Throughput: %.0f ops/sec", opsPerSec)

	stats := sgh.GetStatsWithHistory()
	t.Logf("\nFinal state:")
	t.Logf("  Sessions: %d", stats.TotalSessions)
	t.Logf("  Historical keys: %d", stats.TotalHistoricalKeys)
	t.Logf("  Sessions with history: %d", stats.SessionsWithHistory)
}

// TestHistoryMemoryLeak checks if history grows unbounded
func TestHistoryMemoryLeak(t *testing.T) {
	sgh, _ := NewSessionGeneratorWithHistory(10000)

	t.Log("\n=== Testing for Memory Leaks ===")

	// Scenario: Same identifier keeps getting linked to new ones
	// This simulates a shared device (e.g., public WiFi cookie)
	t.Log("\nScenario: Shared device linking to 10,000 different users")

	start := time.Now()
	for i := 0; i < 10_000; i++ {
		sgh.LinkIdentifiers("device:shared_device", fmt.Sprintf("uid:user_%d", i))
	}
	elapsed := time.Since(start)

	stats := sgh.GetStatsWithHistory()
	key := sgh.GetSessionKey(Identifiers{IdentifierDevice: "shared_device"})
	history := sgh.GetSessionKeyHistory(key)

	t.Logf("Time: %v", elapsed)
	t.Logf("Session size: %d identifiers", sgh.GetSessionSize("device:shared_device"))
	t.Logf("Historical keys count: %d", len(history.OldKeys))
	t.Logf("Total historical keys in system: %d", stats.TotalHistoricalKeys)

	if len(history.OldKeys) > 100 {
		t.Logf("⚠️  WARNING: History growing large (%d keys)!", len(history.OldKeys))
		t.Log("   Consider implementing history pruning/TTL in production")
	}
}

// TestHistoryQueryPerformance tests performance of querying historical keys
func TestHistoryQueryPerformance(t *testing.T) {
	sgh, _ := NewSessionGeneratorWithHistory(10000)

	// Build a session with deep history (10 key changes)
	sgh.GetSessionKey(Identifiers{IdentifierCookie: "abc"})
	for i := 0; i < 10; i++ {
		sgh.LinkIdentifiers(fmt.Sprintf("uid:user_%d", i), "cookie:abc")
	}

	key := sgh.GetSessionKey(Identifiers{IdentifierCookie: "abc"})
	history := sgh.GetSessionKeyHistory(key)

	t.Logf("\n=== History Query Performance ===")
	t.Logf("Session has %d historical keys", len(history.OldKeys))

	// Benchmark GetAllSessionKeys with deep history
	start := time.Now()
	iterations := 100_000
	for i := 0; i < iterations; i++ {
		sgh.GetAllSessionKeys(key)
	}
	elapsed := time.Since(start)

	t.Logf("\nGetAllSessionKeys performance:")
	t.Logf("  Iterations: %d", iterations)
	t.Logf("  Time: %v", elapsed)
	t.Logf("  Avg: %.2f µs/call", float64(elapsed.Microseconds())/float64(iterations))
	t.Logf("  Throughput: %.0f calls/sec", float64(iterations)/elapsed.Seconds())
}

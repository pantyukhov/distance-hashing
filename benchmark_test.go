package distancehashing

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// BenchmarkUnionFind_Find measures the performance of Find operation
func BenchmarkUnionFind_Find(b *testing.B) {
	uf := NewUnionFind()

	// Prepare: create 10,000 elements
	for i := 0; i < 10000; i++ {
		uf.Find(fmt.Sprintf("user_%d", i))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		uf.Find(fmt.Sprintf("user_%d", i%10000))
	}
}

// BenchmarkUnionFind_Union measures the performance of Union operation
func BenchmarkUnionFind_Union(b *testing.B) {
	uf := NewUnionFind()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		uf.Union(fmt.Sprintf("user_%d", i), fmt.Sprintf("user_%d", i+1))
	}
}

// BenchmarkUnionFind_Connected measures the performance of Connected operation
func BenchmarkUnionFind_Connected(b *testing.B) {
	uf := NewUnionFind()

	// Prepare: create linked chain
	for i := 0; i < 10000; i++ {
		uf.Union(fmt.Sprintf("user_%d", i), fmt.Sprintf("user_%d", i+1))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		uf.Connected(fmt.Sprintf("user_%d", i%5000), fmt.Sprintf("user_%d", (i+5000)%10000))
	}
}

// BenchmarkSessionGenerator_GetSessionKey measures session key generation performance
func BenchmarkSessionGenerator_GetSessionKey(b *testing.B) {
	sg, _ := NewSessionGenerator(10000)

	// Prepare some sessions
	for i := 0; i < 1000; i++ {
		sg.GetSessionKey(Identifiers{IdentifierUserID: fmt.Sprintf("user_%d", i)})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sg.GetSessionKey(Identifiers{
			IdentifierUserID: fmt.Sprintf("user_%d", i%1000),
			IdentifierJWT:    fmt.Sprintf("jwt_%d", i),
		})
	}
}

// BenchmarkSessionGenerator_GetSessionKey_CacheHit measures cache hit performance
func BenchmarkSessionGenerator_GetSessionKey_CacheHit(b *testing.B) {
	sg, _ := NewSessionGenerator(10000)

	// Prepare: warm up cache with repeated accesses
	ids := Identifiers{IdentifierUserID: "user_123", IdentifierJWT: "jwt_abc"}
	sg.GetSessionKey(ids)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sg.GetSessionKey(ids)
	}
}

// BenchmarkSessionGenerator_GetSessionKey_CacheMiss measures cache miss performance
func BenchmarkSessionGenerator_GetSessionKey_CacheMiss(b *testing.B) {
	sg, _ := NewSessionGenerator(10000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Each iteration uses a different user ID (cache miss)
		sg.GetSessionKey(Identifiers{IdentifierUserID: fmt.Sprintf("user_%d", i)})
	}
}

// BenchmarkSessionGenerator_LinkIdentifiers measures link operation performance
func BenchmarkSessionGenerator_LinkIdentifiers(b *testing.B) {
	sg, _ := NewSessionGenerator(10000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sg.LinkIdentifiers(
			fmt.Sprintf("uid:user_%d", i),
			fmt.Sprintf("jwt:jwt_%d", i),
		)
	}
}

// BenchmarkSessionGenerator_Concurrent simulates high-concurrency workload
func BenchmarkSessionGenerator_Concurrent(b *testing.B) {
	sg, _ := NewSessionGenerator(10000)

	// Pre-populate with some sessions
	for i := 0; i < 1000; i++ {
		sg.GetSessionKey(Identifiers{IdentifierUserID: fmt.Sprintf("user_%d", i)})
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sg.GetSessionKey(Identifiers{
				IdentifierUserID: fmt.Sprintf("user_%d", i%1000),
				IdentifierJWT:    fmt.Sprintf("jwt_%d", i),
			})
			i++
		}
	})
}

// BenchmarkSessionGenerator_HighThroughput simulates 100K+ RPS scenario
func BenchmarkSessionGenerator_HighThroughput(b *testing.B) {
	sg, _ := NewSessionGenerator(10000)

	// Pre-populate cache with 5000 active users
	for i := 0; i < 5000; i++ {
		sg.GetSessionKey(Identifiers{IdentifierUserID: fmt.Sprintf("user_%d", i)})
	}

	b.ResetTimer()
	b.ReportAllocs()

	// Simulate realistic workload:
	// 70% cache hits (existing users)
	// 30% cache miss (new users)
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			var userID string
			if i%10 < 7 {
				// Cache hit: existing user
				userID = fmt.Sprintf("user_%d", i%5000)
			} else {
				// Cache miss: new user
				userID = fmt.Sprintf("user_%d", 5000+i)
			}

			sg.GetSessionKey(Identifiers{
				IdentifierUserID: userID,
				IdentifierJWT:    fmt.Sprintf("jwt_%d", i),
			})
			i++
		}
	})
}

// BenchmarkSessionGenerator_RealWorldScenario simulates a realistic production scenario
func BenchmarkSessionGenerator_RealWorldScenario(b *testing.B) {
	sg, _ := NewSessionGenerator(10000)

	// Pre-populate with active sessions
	for i := 0; i < 5000; i++ {
		sg.GetSessionKey(Identifiers{
			IdentifierUserID: fmt.Sprintf("user_%d", i),
			IdentifierCookie: fmt.Sprintf("cookie_%d", i),
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Simulate different types of requests:
			switch i % 10 {
			case 0, 1, 2, 3, 4: // 50% - authenticated with cache hit
				sg.GetSessionKey(Identifiers{
					IdentifierUserID: fmt.Sprintf("user_%d", i%5000),
					IdentifierJWT:    fmt.Sprintf("jwt_%d", i),
				})
			case 5, 6: // 20% - cookie-based session
				sg.GetSessionKey(Identifiers{
					IdentifierCookie: fmt.Sprintf("cookie_%d", i%5000),
				})
			case 7: // 10% - new user signup
				sg.GetSessionKey(Identifiers{
					IdentifierUserID: fmt.Sprintf("new_user_%d", i),
					IdentifierEmail:  fmt.Sprintf("user%d@example.com", i),
				})
			case 8: // 10% - link operation (login event)
				sg.LinkIdentifiers(
					fmt.Sprintf("cookie:cookie_%d", i%5000),
					fmt.Sprintf("uid:user_%d", i%5000),
				)
			case 9: // 10% - multi-identifier request
				sg.GetSessionKey(Identifiers{
					IdentifierUserID: fmt.Sprintf("user_%d", i%5000),
					IdentifierJWT:    fmt.Sprintf("jwt_%d", i),
					IdentifierCookie: fmt.Sprintf("cookie_%d", i%5000),
					IdentifierDevice: fmt.Sprintf("device_%d", i%1000),
				})
			}
			i++
		}
	})
}

// Benchmark100KRPS measures throughput to verify 100K+ RPS capability
func Benchmark100KRPS(b *testing.B) {
	sg, _ := NewSessionGenerator(10000)

	// Pre-populate with realistic session count
	for i := 0; i < 5000; i++ {
		sg.GetSessionKey(Identifiers{IdentifierUserID: fmt.Sprintf("user_%d", i)})
	}

	b.ResetTimer()

	// Track operations per second
	var opsCount int64
	const numWorkers = 100

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	// Start workers
	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			defer wg.Done()

			localOps := 0
			for i := 0; i < b.N/numWorkers; i++ {
				userID := fmt.Sprintf("user_%d", (workerID*1000+i)%5000)
				sg.GetSessionKey(Identifiers{
					IdentifierUserID: userID,
					IdentifierJWT:    fmt.Sprintf("jwt_%d", i),
				})
				localOps++
			}

			atomic.AddInt64(&opsCount, int64(localOps))
		}(w)
	}

	wg.Wait()

	// Calculate ops/sec
	opsPerSec := float64(opsCount) / b.Elapsed().Seconds()

	b.ReportMetric(opsPerSec, "ops/sec")
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "req/sec")

	// Verify we achieved 100K+ RPS
	if opsPerSec < 100000 {
		b.Logf("Warning: Achieved %.0f ops/sec, target is 100K+ ops/sec", opsPerSec)
	} else {
		b.Logf("Success: Achieved %.0f ops/sec (> 100K target)", opsPerSec)
	}
}

// BenchmarkMemoryUsage measures memory usage for large session counts
func BenchmarkMemoryUsage(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Sessions_%d", size), func(b *testing.B) {
			sg, _ := NewSessionGenerator(size / 2)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				for j := 0; j < size; j++ {
					sg.GetSessionKey(Identifiers{
						IdentifierUserID: fmt.Sprintf("user_%d", j),
					})
				}

				// Clear for next iteration
				if i < b.N-1 {
					sg.Clear()
				}
			}

			stats := sg.GetStats()
			b.ReportMetric(float64(stats.TotalIdentifiers), "identifiers")
			b.ReportMetric(float64(stats.TotalSessions), "sessions")
		})
	}
}

// BenchmarkPathCompression verifies path compression optimization
func BenchmarkPathCompression(b *testing.B) {
	uf := NewUnionFind()

	// Create a long chain: 0 -> 1 -> 2 -> ... -> 999
	for i := 0; i < 1000; i++ {
		uf.Union(fmt.Sprintf("node_%d", i), fmt.Sprintf("node_%d", i+1))
	}

	b.ResetTimer()

	// First find compresses the path
	b.Run("FirstFind", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			uf.Find("node_0")
		}
	})

	// Subsequent finds should be faster due to path compression
	b.Run("SubsequentFind", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			uf.Find("node_500")
		}
	})
}

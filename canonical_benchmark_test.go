package distancehashing

import (
	"fmt"
	"sync/atomic"
	"testing"
)

// BenchmarkCanonical_GetSessionKey measures performance of canonical session key generation
func BenchmarkCanonical_GetSessionKey(b *testing.B) {
	csg, _ := NewCanonicalSessionGenerator(10000)

	// Prepare some sessions
	for i := 0; i < 1000; i++ {
		csg.GetSessionKey(Identifiers{IdentifierUserID: fmt.Sprintf("user_%d", i)})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		csg.GetSessionKey(Identifiers{
			IdentifierUserID: fmt.Sprintf("user_%d", i%1000),
			IdentifierJWT:    fmt.Sprintf("jwt_%d", i),
		})
	}
}

// BenchmarkCanonical_GetSessionKey_CacheHit measures cache hit performance
func BenchmarkCanonical_GetSessionKey_CacheHit(b *testing.B) {
	csg, _ := NewCanonicalSessionGenerator(10000)

	// Warm up cache
	ids := Identifiers{IdentifierUserID: "user_123", IdentifierJWT: "jwt_abc"}
	csg.GetSessionKey(ids)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		csg.GetSessionKey(ids)
	}
}

// BenchmarkCanonical_GetSessionKey_CacheMiss measures cache miss performance
func BenchmarkCanonical_GetSessionKey_CacheMiss(b *testing.B) {
	csg, _ := NewCanonicalSessionGenerator(10000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Each iteration uses different user ID (cache miss)
		csg.GetSessionKey(Identifiers{IdentifierUserID: fmt.Sprintf("user_%d", i)})
	}
}

// BenchmarkCanonical_LinkIdentifiers measures link operation performance
func BenchmarkCanonical_LinkIdentifiers(b *testing.B) {
	csg, _ := NewCanonicalSessionGenerator(10000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		csg.LinkIdentifiers(
			fmt.Sprintf("uid:user_%d", i),
			fmt.Sprintf("jwt:jwt_%d", i),
		)
	}
}

// BenchmarkCanonical_Concurrent simulates concurrent workload
func BenchmarkCanonical_Concurrent(b *testing.B) {
	csg, _ := NewCanonicalSessionGenerator(10000)

	// Pre-populate
	for i := 0; i < 1000; i++ {
		csg.GetSessionKey(Identifiers{IdentifierUserID: fmt.Sprintf("user_%d", i)})
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			csg.GetSessionKey(Identifiers{
				IdentifierUserID: fmt.Sprintf("user_%d", i%1000),
				IdentifierJWT:    fmt.Sprintf("jwt_%d", i),
			})
			i++
		}
	})
}

// BenchmarkCanonical_HighThroughput simulates 100K+ RPS scenario
func BenchmarkCanonical_HighThroughput(b *testing.B) {
	csg, _ := NewCanonicalSessionGenerator(10000)

	// Pre-populate with 5000 active users
	for i := 0; i < 5000; i++ {
		csg.GetSessionKey(Identifiers{IdentifierUserID: fmt.Sprintf("user_%d", i)})
	}

	b.ResetTimer()
	b.ReportAllocs()

	// 70% cache hits, 30% cache miss
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			var userID string
			if i%10 < 7 {
				// Cache hit
				userID = fmt.Sprintf("user_%d", i%5000)
			} else {
				// Cache miss
				userID = fmt.Sprintf("user_%d", 5000+i)
			}

			csg.GetSessionKey(Identifiers{
				IdentifierUserID: userID,
				IdentifierJWT:    fmt.Sprintf("jwt_%d", i),
			})
			i++
		}
	})
}

// BenchmarkCanonical_RealWorldScenario simulates realistic production usage
func BenchmarkCanonical_RealWorldScenario(b *testing.B) {
	csg, _ := NewCanonicalSessionGenerator(10000)

	// Pre-populate
	for i := 0; i < 5000; i++ {
		csg.GetSessionKey(Identifiers{
			IdentifierUserID: fmt.Sprintf("user_%d", i),
			IdentifierCookie: fmt.Sprintf("cookie_%d", i),
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			switch i % 10 {
			case 0, 1, 2, 3, 4: // 50% - authenticated with cache hit
				csg.GetSessionKey(Identifiers{
					IdentifierUserID: fmt.Sprintf("user_%d", i%5000),
					IdentifierJWT:    fmt.Sprintf("jwt_%d", i),
				})
			case 5, 6: // 20% - cookie-based session
				csg.GetSessionKey(Identifiers{
					IdentifierCookie: fmt.Sprintf("cookie_%d", i%5000),
				})
			case 7: // 10% - new user signup
				csg.GetSessionKey(Identifiers{
					IdentifierUserID: fmt.Sprintf("new_user_%d", i),
					IdentifierEmail:  fmt.Sprintf("user%d@example.com", i),
				})
			case 8: // 10% - link operation
				csg.LinkIdentifiers(
					fmt.Sprintf("cookie:cookie_%d", i%5000),
					fmt.Sprintf("uid:user_%d", i%5000),
				)
			case 9: // 10% - multi-identifier request
				csg.GetSessionKey(Identifiers{
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

// BenchmarkCanonical_100KRPS measures throughput
func BenchmarkCanonical_100KRPS(b *testing.B) {
	csg, _ := NewCanonicalSessionGenerator(10000)

	// Pre-populate
	for i := 0; i < 5000; i++ {
		csg.GetSessionKey(Identifiers{IdentifierUserID: fmt.Sprintf("user_%d", i)})
	}

	b.ResetTimer()

	var opsCount int64

	b.RunParallel(func(pb *testing.PB) {
		localOps := 0
		i := 0
		for pb.Next() {
			userID := fmt.Sprintf("user_%d", (i)%5000)
			csg.GetSessionKey(Identifiers{
				IdentifierUserID: userID,
				IdentifierJWT:    fmt.Sprintf("jwt_%d", i),
			})
			localOps++
			i++
		}
		atomic.AddInt64(&opsCount, int64(localOps))
	})

	opsPerSec := float64(opsCount) / b.Elapsed().Seconds()
	b.ReportMetric(opsPerSec, "ops/sec")

	if opsPerSec < 100000 {
		b.Logf("Warning: Achieved %.0f ops/sec, target is 100K+", opsPerSec)
	} else {
		b.Logf("Success: Achieved %.0f ops/sec (> 100K target)", opsPerSec)
	}
}

// BenchmarkComparison_CacheMiss compares N-Degree vs Canonical on cache miss
func BenchmarkComparison_CacheMiss(b *testing.B) {
	b.Run("NDegree", func(b *testing.B) {
		sg, _ := NewSessionGenerator(10000)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			sg.GetSessionKey(Identifiers{IdentifierUserID: fmt.Sprintf("user_%d", i)})
		}
	})

	b.Run("Canonical", func(b *testing.B) {
		csg, _ := NewCanonicalSessionGenerator(10000)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			csg.GetSessionKey(Identifiers{IdentifierUserID: fmt.Sprintf("user_%d", i)})
		}
	})
}

// BenchmarkComparison_LinkIdentifiers compares link performance
func BenchmarkComparison_LinkIdentifiers(b *testing.B) {
	b.Run("NDegree", func(b *testing.B) {
		sg, _ := NewSessionGenerator(10000)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			sg.LinkIdentifiers(
				fmt.Sprintf("uid:user_%d", i),
				fmt.Sprintf("jwt:jwt_%d", i),
			)
		}
	})

	b.Run("Canonical", func(b *testing.B) {
		csg, _ := NewCanonicalSessionGenerator(10000)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			csg.LinkIdentifiers(
				fmt.Sprintf("uid:user_%d", i),
				fmt.Sprintf("jwt:jwt_%d", i),
			)
		}
	})
}

// BenchmarkComparison_HighThroughput compares realistic workload
func BenchmarkComparison_HighThroughput(b *testing.B) {
	b.Run("NDegree", func(b *testing.B) {
		sg, _ := NewSessionGenerator(10000)
		for i := 0; i < 5000; i++ {
			sg.GetSessionKey(Identifiers{IdentifierUserID: fmt.Sprintf("user_%d", i)})
		}

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				sg.GetSessionKey(Identifiers{
					IdentifierUserID: fmt.Sprintf("user_%d", i%5000),
					IdentifierJWT:    fmt.Sprintf("jwt_%d", i),
				})
				i++
			}
		})
	})

	b.Run("Canonical", func(b *testing.B) {
		csg, _ := NewCanonicalSessionGenerator(10000)
		for i := 0; i < 5000; i++ {
			csg.GetSessionKey(Identifiers{IdentifierUserID: fmt.Sprintf("user_%d", i)})
		}

		b.ResetTimer()
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				csg.GetSessionKey(Identifiers{
					IdentifierUserID: fmt.Sprintf("user_%d", i%5000),
					IdentifierJWT:    fmt.Sprintf("jwt_%d", i),
				})
				i++
			}
		})
	})
}

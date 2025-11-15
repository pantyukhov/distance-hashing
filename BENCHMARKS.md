# Performance Benchmarking Guide

This document describes how to run and analyze performance benchmarks for the Distance Hashing library.

## Overview

The project includes comprehensive performance benchmarks covering:

- **UnionFind operations**: Find, Union, Connected
- **Session Generator**: N-Degree Hash implementation
- **Canonical Session**: Production-optimized algorithm
- **High Throughput**: 100K+ RPS scenarios
- **Memory Usage**: Scalability testing
- **Algorithm Comparison**: N-Degree vs Canonical

## Quick Start

### Run All Benchmarks Locally

```bash
# Simple run
go test -bench=. -benchmem

# Extended run (3 seconds per benchmark)
go test -bench=. -benchmem -benchtime=3s

# With timeout for long-running tests
go test -bench=. -benchmem -benchtime=3s -timeout=30m
```

### Using the Benchmark Script

```bash
# Run comprehensive benchmark suite
./scripts/run-benchmarks.sh
```

This script will:
- Run all benchmark suites
- Generate timestamped result files
- Create a summary report
- Save results to `benchmark-results/` directory

### Run Specific Benchmarks

```bash
# UnionFind operations only
go test -bench=BenchmarkUnionFind -benchmem

# Canonical Session only
go test -bench=BenchmarkCanonical -benchmem

# High throughput tests
go test -bench=Benchmark.*100KRPS -benchmem -benchtime=5s

# Memory usage tests
go test -bench=BenchmarkMemory -benchmem

# Comparison tests
go test -bench=BenchmarkComparison -benchmem
```

## GitHub Actions Integration

### Automatic Performance Testing

Performance benchmarks run automatically on:
- ✅ Push to `main` or `master` branch
- ✅ Pull requests to `main` or `master`
- ✅ Manual workflow dispatch

**Workflow**: [`.github/workflows/performance.yml`](.github/workflows/performance.yml)

Results are uploaded as GitHub Actions artifacts and retained for 30-90 days.

### Benchmark Comparison (PR Reviews)

When you create a pull request, the comparison workflow automatically:
- Runs benchmarks on both base and PR code
- Compares results using `benchstat`
- Detects performance regressions
- Comments on PR if regressions are found

**Workflow**: [`.github/workflows/benchmark-compare.yml`](.github/workflows/benchmark-compare.yml)

### Manual Workflow Trigger

You can manually trigger benchmarks from GitHub Actions:

1. Go to **Actions** → **Performance Benchmarks**
2. Click **Run workflow**
3. Select branch
4. Click **Run workflow**

## Understanding Results

### Benchmark Output Format

```
BenchmarkName-8    1000000    1234 ns/op    567 B/op    8 allocs/op
```

- `BenchmarkName`: Test name
- `-8`: Number of CPU cores used (GOMAXPROCS)
- `1000000`: Number of iterations
- `1234 ns/op`: Nanoseconds per operation (⬇️ lower is better)
- `567 B/op`: Bytes allocated per operation (⬇️ lower is better)
- `8 allocs/op`: Number of allocations per operation (⬇️ lower is better)

### Performance Targets

| Metric | Target | Status Check |
|--------|--------|--------------|
| **Throughput** | 100K+ ops/sec | See `Benchmark.*100KRPS` |
| **Latency p99** | < 1ms (< 1,000,000 ns) | Check `ns/op` values |
| **Memory per session** | < 1 KB (< 1024 B) | See `BenchmarkMemory` |
| **Cache hit latency** | < 100 ns | `BenchmarkCanonical_CacheHit` |
| **Cache miss latency** | < 200 ns | `BenchmarkCanonical_CacheMiss` |

### Using benchstat for Comparison

```bash
# Run benchmarks twice
go test -bench=. -benchmem -count=5 > old.txt
# ... make changes ...
go test -bench=. -benchmem -count=5 > new.txt

# Install benchstat
go install golang.org/x/perf/cmd/benchstat@latest

# Compare results
benchstat old.txt new.txt
```

**Example output**:
```
name                old time/op  new time/op  delta
UnionFind_Find-8    81.5ns ± 2%  79.3ns ± 1%  -2.70%
Canonical_CacheHit  96.3ns ± 1%  94.1ns ± 2%  -2.28%
```

## Advanced Profiling

### CPU Profiling

```bash
# Run with CPU profiling
go test -bench=BenchmarkCanonical_100KRPS \
  -benchtime=10s \
  -cpuprofile=cpu.prof

# Analyze profile
go tool pprof cpu.prof
```

**Interactive commands**:
- `top`: Show top functions by CPU time
- `list FunctionName`: Show source code with annotations
- `web`: Generate SVG call graph (requires Graphviz)

### Memory Profiling

```bash
# Run with memory profiling
go test -bench=BenchmarkCanonical_100KRPS \
  -benchtime=10s \
  -memprofile=mem.prof

# Analyze profile
go tool pprof mem.prof
```

### Trace Analysis

```bash
# Generate execution trace
go test -bench=BenchmarkCanonical_Concurrent \
  -benchtime=5s \
  -trace=trace.out

# Visualize trace
go tool trace trace.out
```

## Continuous Benchmarking

### Setting Up Benchmark Tracking

For long-term performance tracking, consider using:

1. **[benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat)** - Statistical comparison
2. **[go-benchmark-comparison](https://github.com/cespare/prettybench)** - Pretty formatting
3. **Custom dashboard** - Store results in ClickHouse/Prometheus

### Example: Storing Historical Results

```bash
# Tag results with git commit
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date +%Y%m%d_%H%M%S)

go test -bench=. -benchmem > "benchmarks/results_${DATE}_${COMMIT}.txt"

# Track in git
git add "benchmarks/results_${DATE}_${COMMIT}.txt"
git commit -m "benchmark: performance results for ${COMMIT}"
```

## Benchmark Files Reference

| File | Description |
|------|-------------|
| [`benchmark_test.go`](benchmark_test.go) | UnionFind and SessionGenerator benchmarks |
| [`canonical_benchmark_test.go`](canonical_benchmark_test.go) | Canonical Session benchmarks |
| [`scripts/run-benchmarks.sh`](scripts/run-benchmarks.sh) | Automated benchmark runner |
| [`.github/workflows/performance.yml`](.github/workflows/performance.yml) | CI benchmark workflow |
| [`.github/workflows/benchmark-compare.yml`](.github/workflows/benchmark-compare.yml) | PR comparison workflow |

## Interpreting Regression Warnings

When a PR introduces performance regressions, the CI will:
- ❌ Mark checks as failed
- 💬 Comment on PR with details
- 📊 Upload comparison artifacts

### Acceptable Regressions
- < 5% slowdown with significant functionality improvement
- < 10% slowdown with better correctness guarantees
- Memory increase if it enables new features

### Unacceptable Regressions
- > 20% slowdown without clear justification
- > 30% memory increase
- Failure to meet 100K ops/sec target

## Troubleshooting

### Benchmarks are inconsistent

**Problem**: Results vary widely between runs

**Solutions**:
```bash
# Run multiple iterations
go test -bench=. -benchmem -count=10

# Disable CPU frequency scaling (Linux)
sudo cpupower frequency-set --governor performance

# Close other applications
# Use dedicated benchmark machine
```

### Benchmarks take too long

**Problem**: Full suite takes > 10 minutes

**Solutions**:
```bash
# Reduce benchmark time
go test -bench=. -benchmem -benchtime=1s

# Run specific benchmarks
go test -bench=BenchmarkCanonical -benchmem

# Skip long-running tests
go test -bench=. -benchmem -short
```

### Out of memory errors

**Problem**: Large-scale benchmarks crash

**Solutions**:
```bash
# Increase timeout
go test -bench=. -timeout=1h

# Run benchmarks individually
go test -bench=BenchmarkMemoryUsage/Sessions_1000 -benchmem

# Reduce cache size in benchmark code
```

## Best Practices

### Writing New Benchmarks

```go
func BenchmarkMyFeature(b *testing.B) {
    // Setup (not timed)
    generator, _ := NewCanonicalSessionGenerator(10000)

    b.ResetTimer()  // Start timing here
    b.ReportAllocs() // Report allocation stats

    for i := 0; i < b.N; i++ {
        // Code to benchmark
        generator.GetSessionKey(Identifiers{
            UserID: fmt.Sprintf("user_%d", i),
        })
    }
}
```

### Parallel Benchmarks

```go
func BenchmarkConcurrent(b *testing.B) {
    generator, _ := NewCanonicalSessionGenerator(10000)

    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            // This runs in parallel across CPUs
            generator.GetSessionKey(Identifiers{
                UserID: fmt.Sprintf("user_%d", i),
            })
            i++
        }
    })
}
```

### Sub-benchmarks

```go
func BenchmarkScalability(b *testing.B) {
    sizes := []int{1000, 10000, 100000}

    for _, size := range sizes {
        b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
            // Benchmark for specific size
        })
    }
}
```

## Contributing

When contributing performance improvements:

1. ✅ Run benchmarks before and after changes
2. ✅ Use `benchstat` to verify improvements
3. ✅ Document any tradeoffs in PR description
4. ✅ Ensure all benchmarks still pass performance targets
5. ✅ Add new benchmarks for new features

## Resources

- [Go Testing Package](https://pkg.go.dev/testing)
- [Go Benchmark Documentation](https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go)
- [Profiling Go Programs](https://go.dev/blog/pprof)
- [benchstat Tool](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat)

---

**Questions?** Open an issue or discussion on GitHub.

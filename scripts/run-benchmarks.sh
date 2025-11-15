#!/bin/bash

# Performance Benchmark Runner Script
# This script runs all performance benchmarks and generates a comprehensive report

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
RESULTS_DIR="$PROJECT_DIR/benchmark-results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}=========================================${NC}"
echo -e "${BLUE}Distance Hashing Performance Benchmarks${NC}"
echo -e "${BLUE}=========================================${NC}"
echo ""

# Create results directory
mkdir -p "$RESULTS_DIR"

cd "$PROJECT_DIR"

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${YELLOW}Error: Go is not installed${NC}"
    exit 1
fi

echo -e "${GREEN}Go version:${NC} $(go version)"
echo -e "${GREEN}System:${NC} $(uname -s) $(uname -m)"
echo -e "${GREEN}CPU cores:${NC} $(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo "unknown")"
echo ""

# Function to run benchmark with timing
run_benchmark() {
    local name=$1
    local pattern=$2
    local benchtime=${3:-3s}
    local output_file="$RESULTS_DIR/${name}_${TIMESTAMP}.txt"

    echo -e "${BLUE}Running: $name${NC}"
    echo "Output: $output_file"

    go test -bench="$pattern" -benchmem -benchtime="$benchtime" -timeout=30m \
        | tee "$output_file"

    echo -e "${GREEN}✓ Completed${NC}"
    echo ""
}

# Run all benchmark suites
echo -e "${YELLOW}Starting benchmark execution...${NC}"
echo ""

run_benchmark "full-suite" "." "3s"
run_benchmark "unionfind" "BenchmarkUnionFind" "3s"
run_benchmark "canonical" "BenchmarkCanonical" "3s"
run_benchmark "session-generator" "BenchmarkSessionGenerator" "3s"
run_benchmark "throughput-100k" "Benchmark.*100KRPS" "5s"
run_benchmark "comparison" "BenchmarkComparison" "3s"
run_benchmark "memory-usage" "BenchmarkMemory" "3s"

# Generate summary report
SUMMARY_FILE="$RESULTS_DIR/summary_${TIMESTAMP}.md"

echo -e "${BLUE}Generating summary report...${NC}"

cat > "$SUMMARY_FILE" << EOF
# Performance Benchmark Report

**Date**: $(date -u +"%Y-%m-%d %H:%M:%S UTC")
**Git Commit**: $(git rev-parse HEAD 2>/dev/null || echo "N/A")
**Git Branch**: $(git branch --show-current 2>/dev/null || echo "N/A")
**System**: $(uname -s) $(uname -m)
**Go Version**: $(go version)
**CPU**: $(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo "unknown") cores

---

## Benchmark Files

EOF

# List all generated files
for file in "$RESULTS_DIR"/*_${TIMESTAMP}.txt; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        echo "- [$filename]($filename)" >> "$SUMMARY_FILE"
    fi
done

cat >> "$SUMMARY_FILE" << 'EOF'

---

## Performance Targets

| Metric | Target | Importance |
|--------|--------|------------|
| **Throughput** | 100K+ ops/sec | Critical for production load |
| **Latency p99** | < 1ms | User experience |
| **Memory** | < 1 KB/session | Resource efficiency |
| **Cache hit rate** | > 90% | Performance optimization |

---

## Key Benchmarks

### UnionFind Operations
- **Find**: Core connectivity check
- **Union**: Link two identifiers
- **Connected**: Check if two nodes are in same component

### Session Generator
- **GetSessionKey**: Generate session key for identifiers
- **LinkIdentifiers**: Connect identifiers in the graph
- **Cache Hit/Miss**: Performance with/without caching

### Canonical Session
- **Production-optimized**: Priority-based canonical selection
- **Stable session keys**: Same key across reconnections
- **High throughput**: Concurrent access patterns

---

## How to Read Results

Benchmark output format:
```
BenchmarkName-8    iterations    ns/op    B/op    allocs/op
```

- **iterations**: Number of times the test ran
- **ns/op**: Nanoseconds per operation (lower is better)
- **B/op**: Bytes allocated per operation (lower is better)
- **allocs/op**: Memory allocations per operation (lower is better)

---

## Next Steps

1. Review individual benchmark files for detailed results
2. Compare with previous runs to track performance trends
3. Investigate any regressions or unexpected results
4. Profile with `go test -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof`

EOF

echo -e "${GREEN}✓ Summary generated: $SUMMARY_FILE${NC}"
echo ""

# Display results summary
echo -e "${BLUE}=========================================${NC}"
echo -e "${BLUE}Benchmark Execution Complete!${NC}"
echo -e "${BLUE}=========================================${NC}"
echo ""
echo -e "${GREEN}Results location:${NC} $RESULTS_DIR"
echo ""
echo -e "${GREEN}Generated files:${NC}"
ls -lh "$RESULTS_DIR"/*_${TIMESTAMP}* 2>/dev/null || echo "No files generated"
echo ""
echo -e "${YELLOW}View summary report:${NC}"
echo "  cat $SUMMARY_FILE"
echo ""
echo -e "${YELLOW}Or open in browser:${NC}"
echo "  open $SUMMARY_FILE  # macOS"
echo "  xdg-open $SUMMARY_FILE  # Linux"
echo ""

exit 0

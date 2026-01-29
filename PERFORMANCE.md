# Performance Log

Track search performance over time as optimizations are made.

## Benchmark Environment

Document your test environment for reproducible results:

```
Machine: [Your machine, e.g., Apple M3 Pro, 18GB RAM]
Go version: [e.g., go1.21.0]
OS: [e.g., macOS 14.3]
```

## How to Run Benchmarks

```bash
# Run all benchmarks
make bench

# Run and save results with timestamp
make bench-save

# Compare current to baseline
make bench-compare

# Create benchmark vault
./scripts/create-benchmark-vault.sh medium /tmp/ruin-bench
```

## Benchmark Types

| Benchmark | Description |
|-----------|-------------|
| `TagOnly_*` | Search by tags only (`#daily`) |
| `TextSearch_*` | Full-text search (`lorem`) |
| `*_Realistic` | Varied note sizes (40% tiny, 30% small, 20% medium, 10% large) |
| `*_1000` | 1,000 notes |
| `*_5000` | 5,000 notes |
| `*_10000` | 10,000 notes |

---

## Results

### 2025-01-29 - Phase 11: Search Performance Optimizations

**Environment**: Apple M3 Pro, Go 1.21, macOS

**Changes**:
- Concurrent file reading (worker pool, up to 8 goroutines)
- Tag-only optimization (partial file reading, 1KB chunk)
- Early termination for `--limit` without sorting

#### Uniform Notes (small, identical ~300 byte notes)

| Benchmark | main (baseline) | optimized | Improvement |
|-----------|-----------------|-----------|-------------|
| TagOnly_1000 | 19.5ms | 8.4ms | **2.3x faster** |
| TextSearch_1000 | 22.1ms | 11.0ms | **2.0x faster** |
| TagOnly_10000 | 254ms | 103ms | **2.5x faster** |
| TextSearch_10000 | 249ms | 96ms | **2.6x faster** |

#### Realistic Notes (varied sizes: 40% tiny, 30% small, 20% medium, 10% large)

| Benchmark | Time | Memory | Allocations |
|-----------|------|--------|-------------|
| TagOnly_1000_Realistic | 8.9ms | - | - |
| TextSearch_1000_Realistic | 12.1ms | - | - |
| TagOnly_5000_Realistic | 50ms | - | - |
| TextSearch_5000_Realistic | 46ms | - | - |

**Notes**:
- Tag-only search is faster for smaller datasets
- Concurrent I/O provides most benefit (2x+ speedup)
- Partial file reading provides ~10% additional improvement
- Text search competitive at large scale due to OS file caching

---

### Baseline (Pre-optimization)

**Environment**: Apple M3 Pro, Go 1.21, macOS

**Implementation**: Sequential file reading, full file parsing for all queries

| Benchmark | Time |
|-----------|------|
| TagOnly_1000 | 19.5ms |
| TextSearch_1000 | 22.1ms |
| TagOnly_10000 | 254ms |
| TextSearch_10000 | 249ms |

---

## Performance Targets

| Vault Size | Target | Current |
|------------|--------|---------|
| 1,000 notes | < 20ms | 8.9ms |
| 5,000 notes | < 100ms | 50ms |
| 10,000 notes | < 200ms | 103ms |
| 50,000 notes | < 1s | TBD |

## Adding New Results

When making performance-related changes:

1. Run `make bench-save` before changes (baseline)
2. Make your changes
3. Run `make bench-save` after changes
4. Run `make bench-compare` to see diff
5. Add results to this file with date and description

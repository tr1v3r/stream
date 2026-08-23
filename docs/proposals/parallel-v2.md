# Proposal: Parallel Architecture v2 — Stage Fusion, One Pool, Optional Order

Status: **Implemented — Phases 1 & 2 landed (PR #15, PR #16); gates A1–A6 measured and passing. This document is now the authoritative semantics reference.**

## 1. Motivation (measured, v0.2.0)

v1 parallelism spawns **one goroutine pool per parallel-aware operation** (`Filter`/`Map`/`Peek` each call `parallelSeq`). Measured machinery overhead on a near-free workload (`Filter+Map` over 100k ints, Apple M3 Pro):

| mode | wall time | overhead vs serial |
|---|---|---|
| serial | 1.0 ms | 1× |
| `Parallel(2)` (2 pools × 2 workers) | 14.6 ms | **14×** |
| `Parallel(4)` (2 pools × 4 workers) | 18.1 ms | **18×** |
| `Parallel(8)` (2 pools × 8 workers) | 20.9 ms | **21×** |

With near-free user work, `Parallel(n)` is a **large pessimization**. It only wins when per-element work is heavy (≥ ~100µs), which contradicts the API's promise. Secondary problems:

1. **Order is never preserved** — there is no ordered mode.
2. **Parallel coverage is arbitrary** — `Convert`/`FlatMap`/`Sort`/`Reverse`/`Distinct` silently ignore `Parallel(n)`; users cannot tell which stages ran concurrently.
3. **Semantics are surprising** — `Parallel` applies to *subsequent parallel-aware ops* until a stateful op resets it, an invisible and undocumented scope.

## 2. Goals / Non-Goals

**Goals**
- G1: One worker pool per pipeline **run**, not per operation.
- G2: Ordered mode as an opt-in (`Ordered()`), unordered stays default (v1 compat).
- G3: Parallel coverage becomes explicit: all stateless stages inside the parallel section run fused; stateful stages are section boundaries.
- G4: Machinery overhead ≤ 3× serial for near-free work (vs 14–21× today).
- G5: Zero goroutine leaks by construction (reuse the v1 cancel/drain pattern in exactly one place).

**Non-Goals**
- Multi-terminal reuse of pools (streams are single-use; a pool per terminal run is acceptable).
- Parallelizing stateful stages themselves (Sort stays materialize-then-sort).
- Changing serial-mode performance or the lazy closure core.

## 3. Core Design: Stage Fusion with Sectioned Concurrency

### 3.0 Two parallelism models, both supported

Per-op pools (v1) accidentally provided two things at once: **data parallelism** (n workers inside one op) and **pipeline parallelism** (adjacent ops' pools overlap — element 100 filtering while element 50 maps). Fusion keeps only the former. Both matter for different workloads:

- Homogeneous light CPU stages → fusion wins (machinery cost dominates; inter-op channels are pure overhead).
- Heterogeneous stages (IO-heavy then CPU-light) → per-stage sizing + pipeline overlap is the right model; channel cost is negligible against IO latency.

Therefore: **a mid-chain `Parallel(n)` call closes the current section and opens a new one with n workers.** The section boundary flows through a channel — which is exactly what gives adjacent sections pipeline parallelism. Consecutive stateless ops *without* an intervening `Parallel` call fuse into a single function run by one pool.

```go
// Heterogeneous: full v1 expressiveness, sections sized per cost profile,
// A's output flows into B while A is still producing (pipeline parallelism)
stream.SliceOf(urls...).
    Parallel(16).                              // section A: 16 workers for IO waits
    Filter(func(u string) bool { return checkRemote(u) }).
    Parallel(2).                               // closes A, opens B
    Map(func(u string) string { return parse(u) })   // fuses with subsequent same-section ops

// Homogeneous: one Parallel covers the run — full fusion, one pool
stream.SliceOf(nums...).
    Parallel(4).Filter(f).Map(g).Peek(log)
```

### 3.1 Fusable stages

Every stateless op already has the shape `func(T) (T, bool)` (element in → element out + keep flag) or `func(T) T`. While composing the pipeline serially, also build a **fused stage function**:

```
stage := compose(filterFn, mapFn, peekFn)   // single closure, no channels between them
```

`Filter`→`Map`→`Peek` chains become **one function applied by one pool**. No inter-op channels, no per-op goroutines, no reorder ambiguity *within* the fused section.

### 3.2 The executor

```
runParallel(prev iter.Seq[T], stage func(T) (T, bool), workers int, ordered bool) iter.Seq[T]
```

- **Feeder** (1 goroutine): pulls `prev`, emits `(index, element)` into `in`.
- **Workers** (n goroutines): apply `stage`, emit `(index, result)` into `out`.
- **Consumer** (the caller's range): unordered mode yields directly; ordered mode reconstructs sequence order via a slot ring (see 3.3).
- **Shutdown**: exactly the v1 proven pattern — derived cancellable ctx, workers drain `in` after cancel, consumer drains `out` on any exit. One implementation, one leak test.

Batching: feeder sends **batches of 64** (two int64 counts per element of index+value already amortize channel cost; batching reduces it further and gives workers cache locality). Tunable constant, benchmarked in phase 1.

### 3.3 Ordered mode

`Parallel(n)` sections optionally `.Ordered()`:

- Each element carries its input index.
- Consumer fills `slots[i % window]`; yields slot `next` when present; window = `workers × batch` bounded buffer (order only needs a *sliding* window, not full materialization).
- Backpressure is natural: consumer blocks on `next` slot ⇒ feeder pauses ⇒ upstream stages stall.

Cost model (to verify in phase-1 benchmark): ordered adds one slot check + copy per element, no extra goroutines.

### 3.4 Section boundaries

Stateful ops (`Distinct`/`DistinctBy`/`Sort`/`ReverseSort`/`Reverse`/`Limit`/`Skip`/`Pick`/`Execute`) **close** the current parallel section (materialize via the fused stage), then a new section may open. Consequence: `Parallel(4).Sort(cmp)` sorts serially *after* a parallel pre-section — explicit and documented, vs v1's silent ignore.

`FlatMap`/`MapTo` change the element type ⇒ close the section too (fused stage is monomorphic per section). Documented as "type changes end the parallel section".

## 4. API

```go
// v2 semantics: section-scoped parallelism
stream.SliceOf(data...).
    Parallel(4).            // opens a parallel section, 4 workers
    Filter(heavy).          // fused
    Map(heavy).             // fused  ← ONE pool for Filter+Map
    Ordered().              // opt-in: preserve encounter order (no-op in serial mode)
    Sort(cmp)               // closes section (serial materialize+sort)
```

- `Parallel(n ≤ 0)` = no-op (v1 compat).
- A second `Parallel(m)` mid-chain closes the current section and opens a new one with m workers (per-stage sizing preserved; see 3.0).
- `Parallel(n).Ordered()` replaces nothing in v1 (new capability).
- `.Ordered()` without a parallel section = no-op (safe default).

**Compatibility note (breaking-ish):** v1 `Parallel(n)` scoped per-op with an invisible rolling range; v2 scopes per-section with explicit boundaries (next `Parallel` call, stateful op, or type change). Old code chained like `.Parallel(4).Filter(f)` behaves identically; code chaining `.Parallel(4).Filter(f).Map(g)` now fuses both under one pool — strictly closer to intent, but a release-note-worthy change ⇒ **v0.3.0**, not a patch.

## 5. Acceptance Criteria (phase-gated)

| gate | criterion |
|---|---|
| A1 overhead | near-free `Filter+Map` 100k: `Parallel(4)` ≤ 3× serial |
| A2 scaling | 1ms/element work × 100k: `Parallel(4)` ≥ 3× faster than serial |
| A3 order | `Ordered()` output equals serial output, element-for-element (property test, 1k random pipelines) |
| A4 leak | existing leak test passes unchanged; new: ordered-mode + cancel mid-stream |
| A5 regression | full suite + race + 100% statement coverage preserved |
| A6 heterogeneous | IO-sim (200µs/element) `Parallel(16).Filter` + CPU-light `Parallel(2).Map` two-section pipeline ≥ 1.5× faster than single fused `Parallel(4)` — proving per-section sizing still pays |

## 6. Implementation Phases

1. **Phase 1 — executor core**: `runParallel` unordered + fused-stage composition internally wired behind current `Parallel(n)` (no new API). Benchmarks A1/A2. ~250 lines.
2. **Phase 2 — `Ordered()`**: slot ring + public API. Property test A3. ~150 lines.
3. **Phase 3 — boundaries & docs**: section-close semantics for stateful ops, godoc/README/AGENTS updates, release notes. ~100 lines + docs.

Each phase lands as its own PR behind the acceptance gates above.

## 7. Risks & Open Questions

- **R1**: Fusion complicates the closure core (`streamer` gains a fused-stage field). Mitigation: fused stage built *only* when `parallelSize > 0`; serial path untouched.
- **R2**: Ordered-mode window sizing under skewed workloads (one slow batch stalls the window). Mitigation: window = workers×batch with early-yield on contiguous prefix; benchmark skewed case in phase 2.
- **Q1**: Should `Peek` participate in fusion (side effects under concurrency) — yes, same as v1, but document.
- **Q2**: Revisit goleak now that goroutine paths multiply? Per the recorded trade-off: evaluate at phase-2 acceptance.
- **Q3**: Batch size 64 — confirm on amd64 CI runner (Apple Silicon channel performance differs).

## 8. Alternatives Considered

- **Fusion-only (initial draft of this proposal)**: rejected after review — it silently dropped v1's per-stage concurrency sizing, which is the right model for heterogeneous (IO vs CPU) stages; see 3.0 and gate A6.
- **Keep v1, document the overhead**: honest but leaves the API a trap for every light-workload user (the common case).
- **Worker-pool object with explicit `.Pool(p)` lifecycle**: rejected — breaks the declarative one-liner style; pools as user-managed resources invite leaks.
- **Rely on `errgroup` + external composition**: rejects the library's reason to exist (integrated parallel pipelines).

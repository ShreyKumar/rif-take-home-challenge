# Technical Decisions

The guiding principle for this project was proportionality. The problem is small and well defined, so the goal was to solve it correctly, prove it with tests, and be explicit about where the design stops rather than over-building for a scale target the brief only asks me to reason about.

This document covers the reasoning. The architecture diagram is in `docs/requirements.md` and run instructions are in `README.md`.

---

## 1. Backend language and framework

Go, standard library only. The routing, JSON handling, and HTTP server all come from the standard library, so the project carries almost no third-party surface area and compiles to a single static binary.

Go was chosen because the workload is CPU-bound string scanning served over HTTP under potentially high concurrency. That is close to the language's centre of gravity: cheap goroutines per request, predictable performance, and no runtime to provision. Any of the permitted languages would have solved the problem, so this is a fit argument rather than a correctness one.

The detection algorithm lives in its own package with no knowledge of HTTP or storage. That boundary is what makes it testable in isolation and is the main structural decision in the codebase.

**Pros:** cheap goroutines per request fit a CPU-bound, high-concurrency workload, and stdlib-only means almost no third-party surface and a single static binary. Predictable performance with no runtime to provision keeps deployment to shipping one executable, and the algorithm living in its own package — with no knowledge of HTTP or storage — keeps the core logic testable in isolation.
**Cons:** stdlib-only means assembling by hand what a framework provides — safe server defaults, routing conventions, middleware — and the stdlib's permissive defaults (no timeouts, subtree route matching) become the service's defaults unless explicitly overridden.

## 2. Core algorithm

The matrix is scanned once. From each cell, the search looks in four directions only: right, down, down-right, and down-left. The reverse four directions would rediscover the same windows from the opposite end, so omitting them halves the work without changing the result. Each candidate sequence therefore has exactly one starting cell, which means no deduplication step is needed.

The scan exits the moment a second sequence is found, since the brief only asks whether more than one exists, not how many.

This is O(N²) time and O(1) additional space, which is the floor for a problem that must be able to read every cell.

**Ambiguity resolved:** the brief does not say whether overlapping sequences count separately. A row of six identical letters contains three overlapping windows of four. I treat each distinct starting position as a distinct sequence, so that row alone makes a human a mutant. The alternative reading, requiring two non-overlapping sequences, is defensible, but the chosen interpretation matches the example in the brief and is the simpler contract to explain to a caller. Matrices smaller than 4x4 can contain no sequence and return non-mutant immediately.

**Pros:** a single scan with forward-only directions and early exit achieves O(N²) time and O(1) space, with no deduplication step needed.
**Cons:** it rests on resolving the brief's overlap ambiguity in favour of counting overlapping windows; the non-overlapping reading is also defensible. Under this policy a single row of six identical letters is already a mutant verdict, which may surprise anyone expecting two visibly separate sequences.

## 3. Handling invalid input

The brief specifies 200 for mutant and 403 for human. It says nothing about input that is neither.

Malformed DNA returns **400**, not 403: a non-square matrix, rows of unequal length, characters outside A, T, C, G, or an empty array. A 5x6 matrix is a client error, not a non-mutant human. Collapsing the two would return a plausible-looking verdict for input that was never valid, and would hide the caller's bug behind a status code that reads as a successful judgement.

Request bodies are also size-capped and the server sets connection timeouts. Neither is asked for, but a service positioned as high-throughput should not be exhausted by one slow or oversized request.

**Pros:** 400 keeps malformed input distinguishable from a genuine non-mutant verdict, so a caller's bug is never hidden behind a successful-looking judgement.
**Cons:** the brief says nothing about invalid input, so the entire error contract is an interpretation the caller has to learn.

## 4. Storage

SQLite, accessed through a small storage interface that the HTTP handlers depend on rather than depending on SQLite directly.

The data is a flat set of DNA strings with no relations, so a relational server like PostgreSQL would add operational weight for no modelling benefit. SQLite is the right size for the problem as scoped. It is also, unambiguously, the write ceiling of this system, which §8 measures and §9 addresses. That tradeoff was made knowingly rather than discovered afterwards.

**One record per DNA is enforced by a uniqueness constraint in the database**, keyed on a hash of the DNA sequence, not by an application-level check. Application checks lose races; the constraint does not. Two identical submissions arriving simultaneously still produce exactly one row.

**Pros:** SQLite is right-sized for flat, relation-free data, and constraint-based deduplication cannot lose races the way an application-level check can.
**Cons:** it is unambiguously the write ceiling of the system — a limit accepted knowingly, measured in §8, and addressed in §9.

## 5. Statistics endpoint

Counters for mutant and human DNA are maintained in memory, seeded once from the database at startup and incremented only when a genuinely new record is inserted. `GET /stats/` therefore never runs a `COUNT(*)` and answers in constant time regardless of table size. The ratio guards against division by zero when no human DNA has been recorded.

**This is the design's honest limitation.** In-memory counters are process-local. A single instance is correct; two instances behind a load balancer would each report only the traffic they personally saw. The stats endpoint, as built, is not horizontally scalable. Moving the counters to a shared store is the first thing §9 changes, and it is a deliberate scope decision, not an oversight.

**Pros:** stats answer in constant time regardless of table size, with no `COUNT(*)` on the hot path.
**Cons:** the counters are process-local, so the endpoint as built is correct for exactly one instance and blocks horizontal scaling until they move to a shared store.

## 6. Frontend

Vanilla JavaScript, hand-written CSS, no framework and no build step.

The interface is one text input, one submit action, and one result state. There is no client-side state to manage, no routing, and no component reuse. A framework here would mean a build pipeline and a dependency tree in exchange for nothing the page needs. This is a scope judgement about a trivial UI, not a claim that frameworks or TypeScript carry runtime cost.

The frontend calls the same public API a third-party client would, so it exercises the real contract rather than a privileged path.

**Pros:** no build pipeline or dependency tree for a one-input, one-action UI, and the frontend exercises the same public API any third-party client would.
**Cons:** the judgement only holds while the UI stays trivial; a larger interface would need the tooling this one deliberately avoids.

## 7. Testing

Tests cover the algorithm, the HTTP handlers, and the storage layer, with the coverage gate enforced in CI so a regression below the threshold fails the build rather than being noticed later.

Coverage is weighted toward the algorithm, where correctness actually lives: the mutant and non-mutant examples from the brief, each of the four directions independently, boundary matrices smaller than 4x4, overlapping sequences per the policy in §2, and every rejected-input case in §3. Handler tests assert status codes against the contract. Concurrent writes are tested against the uniqueness constraint directly.

The frontend is not covered by automated tests. For a single form with one interaction, the cost of that harness is not justified, and saying so is more useful than reporting a coverage number that excludes it silently.

**Pros:** coverage concentrates where correctness lives — the algorithm, the status-code contract, and concurrent writes — and the CI gate turns regressions into build failures.
**Cons:** the frontend carries no automated tests; the gap is deliberate and stated, but it is still a gap.

## 8. Load testing

A small load driver built on the standard library is included so the benchmark runs with no tooling beyond the Go toolchain already required to build the project.

The results are single-machine, loopback numbers and are not offered as evidence about the brief's 100 to 1M req/s figure. What they do is confirm a prediction rather than produce a headline: reads scale roughly two orders of magnitude beyond writes, and writes plateau precisely where SQLite's single-writer behaviour says they should. The bottleneck named in §4 is measured, not assumed, and §9 is aimed directly at it.

Reported numbers include hardware, concurrency level, duration, payload, and p50/p95/p99 latency, since throughput without percentiles says very little about behaviour under load.

**Pros:** the harness needs nothing beyond the Go toolchain, and it confirms the predicted single-writer bottleneck with measurements rather than assumptions.
**Cons:** the numbers are single-machine, loopback figures and say nothing about the brief's 100-to-1M req/s range.

## 9. Scalability, designed rather than built

The brief asks me to consider aggressive traffic fluctuation, not to build for it. What follows is the design, explicitly not implemented.

**What already holds up:** the request path keeps no state in the process, the algorithm is constant-space and exits early, deduplication happens at the storage layer so repeated DNA never becomes repeated writes, and stats are O(1) rather than a table scan.

**What has to change:**

1. **Counters move out of process** into a shared store. This is the blocker for running more than one instance, and it comes first.
2. **The storage interface gets a PostgreSQL implementation** with a real connection pool, removing the single-writer ceiling measured in §8. No handler and no line of the algorithm changes, which was the point of the interface.
3. **Writes become asynchronous** behind a queue, with a cache absorbing duplicate DNA before it reaches the database. The mutant verdict is pure computation and never needs to wait on persistence.
4. **Instances scale horizontally** behind a load balancer, since the binary is static and identical everywhere.

At 1M req/s the real questions are rate limiting, autoscaling policy, cache invalidation, and infrastructure-as-code. None of those are in this repository, and pretending otherwise would be worse than saying so.

**Pros:** the scale path is staged and concrete — shared counters, then PostgreSQL behind the existing interface, then async writes — with no handler or algorithm changes required.
**Cons:** none of it is implemented, and the hardest problems at 1M req/s (rate limiting, autoscaling, cache invalidation) are explicitly outside this repository.

## 10. Repository structure

Two top-level directories, `frontend/` and `backend/`, kept separate rather than nesting one inside the other. The seam is the HTTP contract in §3, and keeping the tree flat makes that boundary visible from the repository root.

**Pros:** the frontend/backend seam is the HTTP contract, and the flat tree keeps that boundary visible from the repository root.
**Cons:** the two-folder description has to be kept honest as tooling grows — `loadtest/` (§8) already sits beside them.

## 11. Deployment

Deployed to Render as a container. Vercel was the other candidate and was ruled out on persistence, not on language support.

Vercel runs Go, so that was never the issue. The issue is that Vercel is serverless: each invocation gets a fresh, ephemeral filesystem that is discarded when the invocation ends. A file-backed database cannot survive that. The brief requires one durable record per DNA, and a platform that deletes the database file between requests cannot deliver it. Making Vercel work would have meant moving to a hosted database and paying that operational cost to satisfy a constraint the platform imposed rather than one the problem had.

Render runs a long-lived container with a persistent disk attached, which is the minimum the storage design in §4 requires. The database file lives on that disk and survives redeploys. That is the entire decision.

The image builds from a fully static binary with no C dependencies, so the container ships almost nothing beyond the executable itself.

**Pros:** a long-lived container with a persistent disk is the minimum the file-backed storage design requires, and the database survives redeploys.
**Cons:** the choice is dictated by SQLite's persistence needs — Vercel was viable on language but would have forced a hosted database.

## 12. AI tooling

AI assistance was used for implementation and for automated code review on pull requests. Every architectural decision in this document, and in particular the algorithm's overlap policy, the error contract, and the choice to leave the counters process-local, was made and defended by me. The tests are the contract that decides whether the generated code was right.

**Pros:** AI accelerated implementation and PR review while every architectural decision remained human-made and defended.
**Cons:** the tests are the arbiter of generated code, so that assurance extends only as far as the tests themselves reach.

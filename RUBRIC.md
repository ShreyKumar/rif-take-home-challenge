# RIF Mutant-Detector — Evaluation Rubric

> **Important context:** The RIF technical-test PDF contains **no rubric, no weights, and no
> ranking of its own** — it lists 9 deliverables as a flat "Challenges" bullet list. This document
> is a *synthesis*: every criterion traces back to an actual PDF requirement, but the **weights,
> the ranking, and the grade bands are my own inference**, not stated by RIF. Items not grounded in
> the PDF are explicitly labelled *(inferred)*.
>
> Use it to guide the build and to self-score before submitting.

---

## Ranked summary (most → least important)

Ranked primarily by weight, with "is it a hard gate?" and "does the PDF stress it in words?" as
tie-breakers. Weights total 100.

| Rank | Category | Weight | Cumulative | Type | Why it ranks here |
|---:|---|---:|---:|---|---|
| **1** | Core algorithm correctness (A) | 22 | 22% | 🚪 Gate | Largest single weight; every downstream feature (API, DB, stats) is wrong if this is wrong. |
| **2** | `/mutant/` endpoint (C) | 13 | 35% | 🚪 Gate | The product the PDF centers on; must exist before D/E can run. A service must *work* before coverage means anything. |
| **3** | Tests + >80% coverage (F) | 13 | 48% | ⭐ Emphasized | Explicitly demanded in words; heavily weighted — but it *verifies* functionality, so sits just under it. Tied in weight with #2. |
| **4** | Database + dedup (D) | 10 | 58% | 🚪 Gate | Required, and `/stats/` is only correct if "1 record per DNA" is truly enforced. |
| **5** | `/stats/` endpoint (E) | 9 | 67% | ✅ Required | Core functional deliverable; logically downstream of the DB. |
| **6** | Algorithm efficiency (B) | 8 | 75% | ⭐ Emphasized | The only other item stressed in words. Punches above 8 pts — but a correct-yet-slow algorithm still works, so it's below the gates. |
| **7** | Scalability design (G) | 6 | 81% | 📐 Design | Grounded in the "100–1M req/s" line; graded as *reasoning* in the diagram/README, not a benchmark. |
| **8** | Engineering quality (K) | 6 | 87% | 🧹 Quality | Diffuse, and *inferred* — not in the PDF — so it sits below every PDF-grounded item of equal weight. |
| **9** | Frontend (H) | 5 | 92% | 📦 Deliverable | Low weight, but a **binary required deliverable** — cheap to build, conspicuous if missing. |
| **10** | Architecture diagram (I) | 4 | 96% | 📦 Deliverable | Small points; absence reads as "incomplete." |
| **11** | README (J) | 4 | 100% | 📦 Deliverable | Lowest weight, yet the **first file a grader opens** — trivial to do, costly to omit. |

**Where the points concentrate:** the top 5 ranks (A, C, F, D, E) = **67%** of the grade — the
algorithm, the two endpoints, and tests. Nail those cleanly and you're in strong-pass territory.

### ⚠️ Rank ≠ do-last
Ranks 9–11 (frontend, diagram, README) are low-weight but **binary and required**. Each is cheap to
produce and conspicuous when absent. Skipping a required deliverable to squeeze more out of a
high-rank item is a bad trade — a grader notices "no README" far more than "algorithm could be 5%
faster." *Importance* (this ranking) and *build order* are different questions.

---

## Detailed criteria (in ranked order)

### 1. Core algorithm correctness — 22 pts · 🚪 Gate
`isMutant(String[] dna)` — a human is a mutant if there is **more than one** sequence of four
identical letters, horizontally, vertically, or diagonally.

| Sub-criterion | Full credit | Common deductions |
|---|---|---|
| All 3 orientations | Detects horizontal, vertical, **and both diagonals** (↘ and ↙) | Missing anti-diagonal (↙) — the most common bug |
| "More than one" threshold | Requires **≥ 2** four-runs; a single 4-run → **not** mutant | Using `≥ 1` (returns true on one sequence) |
| Overlap policy | Handles `"AAAAA"`-style runs and **documents** the chosen convention | Undocumented/inconsistent behaviour on overlaps |
| Small / degenerate input | `N < 4` can't be mutant; empty / `1×1` don't crash | Index-out-of-range on small grids |
| Alphabet + shape | Only `A/T/C/G`; enforces or documents **N×N** | Accepts ragged rows or illegal chars silently |

### 2. `/mutant/` REST endpoint — 13 pts · 🚪 Gate
| Full credit | Common deductions |
|---|---|
| `POST /mutant/` parses `{"dna":[...]}` | Wrong path / method |
| **200** for mutant, **403** for human — exactly | 200/200, or 400 used for "human" |
| Rejects malformed/invalid body with **400** *(good practice — PDF only mandates 200/403)* | 500 / panic on bad JSON |

### 3. Automated tests + >80% coverage — 13 pts · ⭐ Emphasized
Full credit: unit tests covering all directions + the edge cases in criterion 1; endpoint /
integration tests for 200/403, stats math, and dedup; **measured coverage > 80%**; assertions test
*behaviour*, not just line execution. Deduct for coverage padding, or no endpoint tests. Coverage is
**CI-gated at ≥ 80% on every PR that touches the backend** (`backend/**`; the frontend is verified manually).

### 4. Database + dedup — 10 pts · 🚪 Gate
Full credit: every verified DNA persisted, **exactly one record per distinct DNA** (UNIQUE key on
the sequence or its hash), and **safe under concurrent duplicate submits** (race → still one row,
via constraint / upsert). Deduct for duplicate rows, or dedup that only works single-threaded.

### 5. `/stats/` endpoint — 9 pts · ✅ Required
Full credit: returns exactly `count_mutant_dna`, `count_human_dna`, `ratio`; **ratio = mutants ÷
humans** (matches the `40/100 = 0.4` example); handles **0 humans** without dividing by zero;
numbers stay consistent with the deduped store. Deduct for `mutants ÷ total`, or a crash at zero.

### 6. Algorithm efficiency — 8 pts · ⭐ Emphasized
Full credit: **early-exit** the instant the 2nd sequence is found; a single in-place scan (no
repeated matrix copies / transposes); ~`O(N²)` time, `O(1)–O(N)` extra space. Deduct for scanning
the whole grid after the answer is known, or per-row regex that rescans needlessly.

### 7. Scalability design (on paper) — 6 pts · 📐 Design
Full credit: the docs (`TECHNICAL_DECISIONS.md` §11) show real reasoning about the 100–1M burst —
**stateless** services, **horizontal scaling**, dedup / cache to shield the DB, **O(1) stats via
counters** (not `COUNT(*)`), optional async / queue for writes — clearly marked as *future work, not
implemented*, with **honest trade-offs**. Graded on the *design narrative*, not a benchmark.

### 8. Engineering quality — 6 pts · 🧹 Quality *(inferred — not in the PDF)*
Full credit: clean separation (algorithm ↔ handlers ↔ storage), graceful error handling, readable
naming, sensible commit history.

### 9. Frontend — 5 pts · 📦 Deliverable
Full credit: input DNA rows, submit to the API, clearly show **mutant vs human**, basic input
feedback. Simplicity is fine — polish is not required.

### 10. Architecture diagram — 4 pts · 📦 Deliverable
Full credit: legible diagram of components (client → API → cache/queue → DB) and the flow for both
endpoints.

### 11. README — 4 pts · 📦 Deliverable
Full credit: build / run / test steps reproducible from a clean checkout; endpoints documented with
example request / response.

---

## ⚠️ Gotchas that quietly cost points
The subtle ones a grader will probe specifically:

1. **Anti-diagonal (↙) missed** — very common; costs correctness.
2. **Threshold off-by-one** — a *single* 4-run wrongly flagged as mutant.
3. **`403`, not `400`, for a valid non-mutant** — 400 means "bad request," which a healthy human isn't.
4. **Ratio = mutant ÷ human** (per the example), not mutant ÷ total.
5. **Dedup under concurrency** — two identical DNAs racing must still yield one row.
6. **Divide-by-zero** in `/stats/` when no humans exist yet.

## Suggested grade bands *(inferred)*
- **90–100** — all 9 delivered, correct, tested, with a credible scale story.
- **70–89** — works end-to-end; gaps in coverage, edge cases, or the scale write-up.
- **50–69** — core algorithm + one endpoint, but missing DB/stats/tests or has correctness bugs.
- **< 50** — algorithm incorrect or major deliverables absent.

---

## Self-scoring checklist
- [ ] `isMutant` detects horizontal, vertical, ↘ and ↙ diagonals
- [ ] Requires **> 1** sequence (a single 4-run is *not* mutant)
- [ ] Overlap convention chosen and documented
- [ ] Handles `N < 4`, empty, ragged, and non-`ATCG` input without crashing
- [ ] Algorithm early-exits at the 2nd sequence
- [ ] `POST /mutant/` → **200** mutant / **403** human
- [ ] Invalid input → **400**
- [ ] DB stores exactly **1 record per DNA**, safe under concurrency
- [ ] `GET /stats/` returns `count_mutant_dna`, `count_human_dna`, `ratio` (mutant ÷ human)
- [ ] `/stats/` handles 0 humans (no divide-by-zero)
- [ ] Automated tests, coverage **> 80%**
- [ ] Frontend inputs DNA and shows the result
- [ ] Architecture diagram present
- [ ] README with build / run / test instructions
- [ ] Scalability reasoning documented

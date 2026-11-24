# Wiki benchmark questions

<!-- TEMPLATE: replace these example questions -->

This file is yours. `wiki-engine init` and `sync-prompts` never overwrite it.

**Before you run the benchmark:** replace the 3 example questions below with
7 real questions about this repository. Then delete the marker line above.
The skill and the headless runner refuse to run while that line is in the file.

## Format

Each question has one block that starts with `## Q<number> — <category>`.

Categories:

- `code`: the code alone can answer it.
- `why`: the answer is a reason or history (code comments, git, or the wiki).
- `both`: the answer needs the code and the operating context.

Each block has four fields, in this order:

- `**Question:**` one question that ends with `?`.
- `**Key points:**` a numbered list. Each point is a fact that a correct answer states.
- `**Typical wrong claims:**` plausible false facts, separated by `;`.
- `**Sources:**` the files, functions or commits that prove the key points.

Rules for good questions:

1. **Check each key against the code and git history, not against the wiki.**
   If you copy a key from the wiki, the test only shows that the wiki agrees with itself.
2. Pick questions that the wiki claims to answer. Include at least one
   question that the code answers alone, and one whose premise is false.
3. Keep each key point short and checkable. Use 2 to 4 key points for each question.
4. When the code changes, check every key again before a run.

Grade each answer against the key points. A **wrong claim** is a stated fact that
the key or the code contradicts. Wrong claims are counted separately. They do
not remove key points.

---

## Q1 — code (EXAMPLE, replace)
**Question:** Which HTTP status code does the `orders` service return when a
client sends an order with a negative quantity, and in which file is this
check?

**Key points:**
1. The service returns 422 (not 400).
2. The check is in `src/orders/validate.py`, in `validate_quantity`.

**Typical wrong claims:** the service returns 400; the check is in the HTTP handler.

**Sources:** `src/orders/validate.py` (`validate_quantity`).

---

## Q2 — why (EXAMPLE, replace)
**Question:** Why does the `orders` service write each order to a queue before
it saves the order to the database?

**Key points:**
1. The database was too slow during sales peaks.
2. The queue lets the service accept the order first and save it later.
3. A worker retries a failed save, so no order is lost.

**Typical wrong claims:** the queue exists for audit logging; the service
writes to the database first.

**Sources:** `src/orders/enqueue.py` (comment above `enqueue_order`); commit `abc1234`.

---

## Q3 — both (EXAMPLE, replace)
**Question:** A developer changes `src/orders/pricing.py`. Which job must
they run before they merge, and what does it check?

**Key points:**
1. They run `make price-check`.
2. It compares the new prices with the stored price list and fails on a drift of more than 1%.

**Typical wrong claims:** `make test` runs the price check; the check never fails the build.

**Sources:** `Makefile` (`price-check`); `.github/workflows/ci.yml`.

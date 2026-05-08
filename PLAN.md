# Ausfallplan Notifier — Plan

A small serverless tool that polls the school's cancellation page on a schedule, detects new entries that may concern my children (3d, 6b), and pushes a notification to my phone via ntfy.sh.

Source page: https://stechlinsee-grundschule.de/ausfall-plan/

## Design summary (decided)

| Concern | Decision |
| --- | --- |
| Language | Go (keep existing scaffolding, refactor as needed) |
| Notification | ntfy.sh (HTTP POST, free, push to phone) |
| Cloud | AWS — Lambda + EventBridge Scheduler + DynamoDB |
| Memory store | DynamoDB single-item table holding the last seen snapshot (hash + entries) |
| Filter strategy | Blacklist of irrelevant classes (`1a`–`6d` excluding `3d` and `6b`). Over-notify on ambiguity; grow the blacklist over time. |
| Sources | Both `tablepress-1` (Ausfallplan) and `tablepress-2` (Aktuelle Informationen) |
| Schedule (Europe/Berlin) | Mornings: every 10 min, 06:50–08:00, Mon–Fri. Evenings: every 30 min, 18:00–21:00, Sun–Thu. |

### Filter semantics

- A specific-class token has the form `<year><letter>` where year ∈ {1..6} and letter ∈ {a..d}, optionally with whitespace (e.g. `3d`, `3 D`, `6 b`).
- An entry's `Klasse` field is tokenized (split by `,` `/` whitespace) and each specific-class token is checked.
- An entry is **dropped** only if it contains ≥1 specific-class token AND every specific-class token is in the blacklist.
- If the field contains anything we don't recognize (`alle Klassen`, `JüL 3`, `3. Klassen`, `Geige`, year-only references, free text, empty, etc.), the entry is **kept** (over-notify).
- Aktuelle-Informationen entries are always kept (free text, no class field).

### Notification content

Each new (changed/added) entry produces one ntfy message:

- Title: e.g. `Neuer Eintrag: 3d` or `Aktuelle Information`
- Body: `Mo, 06.02.2023 · 6. Stunde · 3d · Englisch` (Ausfallplan) or the free text (Aktuelle Informationen)
- Tag: `school`

A single Lambda invocation may emit multiple messages.

### "Changed" detection

- Snapshot = stable canonical-JSON representation of `[]Entry` (Ausfall) + `[]Info` (Aktuelle Informationen) sorted deterministically.
- Compare new snapshot with the one in DynamoDB. Set diff yields *added* entries (we only notify on additions, not removals — a removed cancellation isn't actionable).
- After a successful run (and successful notification of any added items), write the new snapshot back.

## Repository layout (target)

```
ausfallplan/         # parsing, filtering — pure logic, no I/O
fetch/               # HTTP client for the live page
store/               # snapshot persistence (local file + DynamoDB impls behind one interface)
notify/              # ntfy.sh client behind a Notifier interface
internal/run/        # orchestration: fetch → parse → diff → filter → notify → persist
cmd/local/           # local CLI entry point (replaces current main.go)
cmd/lambda/          # AWS Lambda entry point
infra/               # IaC (decided in M6)
PLAN.md              # this file
```

## Milestones

Each milestone ends in a runnable, tested checkpoint. Stop and resume freely. Each milestone is implemented by delegating to a Sonnet sub-agent with the milestone brief; this orchestrator (Opus) reviews and commits.

### M0 — Cleanup & scaffold ✅

**Goal:** Bring the repo into the new shape before behavior changes, so subsequent milestones aren't muddled by legacy code paths.

**Existing artefacts and their fate:**

| Path | Action |
| --- | --- |
| `main.go` | Delete; replaced by `cmd/local/main.go` (added in M1) |
| `ausfallplan/ausfallplan.go` (`GetAllEntries` / `GetEntriesFor`) | Delete; pipeline lives in `internal/run` from M1 onward |
| `ausfallplan/fetch.go` | Move to `fetch/` package; rewrite in M1 to return errors instead of `log.Fatal` |
| `ausfallplan/parse.go` | Keep for now (passing tests); rewritten with goquery in M1 |
| `ausfallplan/parse_test.go`, `filter_test.go` | Keep — guard rails during refactor |
| `ausfallplan/simple_table.html`, `empty_table.html` | Move to `ausfallplan/testdata/` |
| `ausfallplan.html` (root, 35KB) | Move to `ausfallplan/testdata/live_snapshot.html` for full-page parser tests |
| `config.json` (level/class shape) | Delete; new blacklist-shape config arrives in M3 |
| `docker-compose.yml` (MinIO) | Delete; replaced by DynamoDB Local in M5 |
| `minio/` directory (gitignored volume) | Delete from disk |
| `.env`, `.env.example` (S3_* keys) | Strip S3_* keys; keep only `AUSFALL_URL` for now |
| `go.mod` / `go.sum` | Drop `minio-go`, `godotenv` stays |

**Tasks:**
- [ ] Delete the files marked Delete above; move the ones marked Move.
- [ ] `go mod tidy` after removing minio imports; existing tests still compile and pass.
- [ ] `go test ./...` green.
- [ ] Single commit: "Clean up legacy structure".

**Acceptance:**
- `git ls-files` shows only files that have a place in the target layout.
- `go test ./...` passes.
- The repo builds (`go build ./...`) — though it produces no useful binary yet, since `main.go` is gone. M1 reintroduces an entry point.

### M1 — Local fetch + parse, both tables ✅

**Goal:** Application runs locally, fetches the live page, parses both `tablepress-1` and `tablepress-2` into typed entries.

**Tasks (TDD):**
- [x] Switch HTML parsing from regex to `goquery` (more robust against whitespace/attribute changes).
- [x] Add `parse_test.go` cases for `tablepress-2` ("Aktuelle Informationen") with fixture HTML (empty, single, multiple rows).
- [x] Extend parse to return `Snapshot{ Entries []Entry; Infos []Info }`.
- [x] Promote `fetch_page` to a real `Fetch(url)` returning `([]byte, error)` (no `log.Fatal`).
- [x] CLI command `cmd/local fetch` prints the parsed snapshot.

**Acceptance:**
- `go test ./...` green.
- `go run ./cmd/local fetch` against the live URL prints both Ausfall entries and Aktuelle Informationen.

### M2 — Local diffing against last snapshot ✅

**Goal:** Detect added entries vs. the previous run, using a local file as memory.

**Tasks (TDD):**
- [x] `store.Store` interface: `Load() (Snapshot, error)`, `Save(Snapshot) error`.
- [x] `store.FileStore` writes `state.json` (gitignored).
- [x] Canonical serialization for stable equality (sort entries, normalize whitespace).
- [x] `diff.Added(prev, next Snapshot) []Change` — returns added Ausfall entries and added Infos.
- [x] Tests: identical snapshots → no changes; new entry → returned; removed entry → ignored; reordered → no change.
- [x] CLI command `cmd/local check` runs the full pipeline and prints additions.

**Acceptance:**
- Running `check` twice in a row prints additions on first call (vs. empty state) and nothing on second call.

### M3 — Blacklist filter ✅

**Goal:** Drop additions that exclusively concern blacklisted classes.

**Tasks (TDD):**
- [ ] `config.json` schema:
  ```json
  {
    "blacklist": ["1a","1b","1c","1d","2a","2b","2c","2d","3a","3b","3c","4a","4b","4c","4d","5a","5b","5c","5d","6a","6c","6d"]
  }
  ```
- [ ] Tokenizer recognizes specific-class tokens (`\b[1-6]\s*[a-dA-D]\b`) inside the `Klasse` field.
- [ ] Filter rule: drop iff ≥1 recognized token AND all recognized tokens are blacklisted.
- [ ] Tests covering: pure 3d → keep; pure 1a → drop; "1a, 3d" → keep; "3. Klassen" → keep; "alle Klassen" → keep; "Geige" → keep; "" → keep; "1a, 2b" → drop.
- [ ] Aktuelle-Informationen entries bypass the filter.
- [ ] Wire filter into `cmd/local check`.

**Acceptance:**
- Tests pass. Manual run against current page yields plausible filtered output.

### M4 — ntfy.sh notification ⏳

**Goal:** Push real notifications to phone for each surviving addition.

**Tasks (TDD):**
- [ ] `notify.Notifier` interface; `notify.Ntfy` impl; `notify.Stub` for tests.
- [ ] Configurable topic via env (`NTFY_TOPIC`, optional `NTFY_SERVER` defaulting to `https://ntfy.sh`).
- [ ] Dry-run flag for local testing.
- [ ] Pipeline: each addition → one ntfy message; failure to notify must NOT update the snapshot (so we retry next run).
- [ ] Tests with httptest server verifying request shape (Title, Tags, body).

**Acceptance:**
- Subscribe to topic in ntfy.sh app on phone, run `cmd/local check` after a fixture mutation, receive push.

**Open question:** ntfy topic name — pick something unguessable (random suffix). Will record in `.env` only.

### M5 — DynamoDB-backed memory ⏳

**Goal:** Replace the file store with DynamoDB so it works from Lambda.

**Tasks (TDD):**
- [ ] `store.DynamoStore` implementing the same interface.
- [ ] Single-item table `ausfallplan-state` (PK: `id="snapshot"`), one attribute `payload` containing the canonical JSON.
- [ ] Local integration test against DynamoDB Local (in `docker-compose.yml`, replacing or alongside MinIO).
- [ ] Selection of store implementation by env (`STATE_BACKEND=file|dynamo`).

**Acceptance:**
- `cmd/local check` works against DynamoDB Local. Snapshot survives across runs.

### M6 — Deploy as AWS Lambda ⏳

**Goal:** Manual `make deploy` puts the function in AWS, runnable by hand.

**Tasks:**
- [ ] `cmd/lambda` entry using `aws-lambda-go`.
- [ ] Choose IaC: AWS SAM (template.yaml) — simple, official, free. Decision recorded here once tried.
- [ ] Resources: Lambda (Go arm64), DynamoDB table, IAM role, Lambda env vars (`AUSFALL_URL`, `NTFY_TOPIC`, `STATE_BACKEND=dynamo`, `DDB_TABLE`).
- [ ] `make deploy` builds + `sam deploy`.
- [ ] Manual invocation: `aws lambda invoke ...` produces same behavior as local.

**Acceptance:**
- One manual invoke against an empty table sends notifications for all current entries; second invoke sends nothing.

### M7 — Graceful errors + self-notification ⏳

**Goal:** Any unexpected failure (network, parse, DB, ntfy) ends the invocation cleanly and pings me.

**Tasks (TDD):**
- [ ] No more `log.Fatal` / `panic` in production paths; replace with `error` returns.
- [ ] Top-level handler in `cmd/lambda` catches errors and posts to ntfy with priority `urgent` and tag `warning`, then returns the error so CloudWatch shows red.
- [ ] Distinguish: structural parse failure (HTML changed) vs. transient network failure (don't spam — only notify on parse/persist failures, just log on transient HTTP errors with retry).
- [ ] Lambda timeout set to 30s; ntfy/HTTP clients have explicit timeouts < that.
- [ ] Tests for the error-notification path with `notify.Stub`.

**Acceptance:**
- Forcing a parse error locally triggers an "urgent" ntfy message.

### M8 — EventBridge schedule ⏳

**Goal:** Hands-off operation on the agreed cadence.

**Tasks:**
- [ ] Two EventBridge Scheduler schedules in Europe/Berlin:
  - `morning`: cron `0/10 6-8 ? * MON-FRI *` filtered to 06:50–08:00 — actually expressed as two schedules or a `cron(50,0/10 6-7 ...)` style. Final cron decided during implementation.
  - `evening`: cron `0/30 18-21 ? * SUN-THU *`.
- [ ] Both target the same Lambda alias.
- [ ] `make deploy` provisions them.

**Acceptance:**
- CloudWatch shows scheduled invocations at the expected times for at least one full cycle.

## Working agreements

- TDD: red → green → refactor for every behavior change. No "we'll add tests later".
- Each milestone gets its own commit (or PR). Mark the checkbox in this file when truly done.
- Update this file in the same commit when scope changes — it's the single source of truth.
- Sub-agent execution: orchestrator (Opus) writes the milestone brief and reviews; Sonnet sub-agents do the implementation in isolation.

## Status legend

- ⏳ Not started
- 🔧 In progress
- ✅ Done

# slam — v1 Backlog

32 atomic items across 5 milestones. Work top-to-bottom within a milestone; deps marked when they cross items.

## M1 — Walking skeleton

Goal: `slam -n 100 https://example.com/` runs end-to-end and prints a sensible summary.

- [x] **M1.1 — Repo init.** Module path, package layout (`cmd/`, `internal/`, `engine/`), `.gitignore`, MIT license, Makefile, README stub, this backlog. Deps: —. Done when: `go build ./...` passes.
- [x] **M1.2 — Engine: core types.** `Target`, `Plan`, `Result`, `Snapshot`, `Summary`, `LatencyStats`, `Reporter` interface in package `engine`. Stub `Runner.Run()` returning empty `Summary`. Deps: M1.1. Done when: types compile, stub runner returns.
- [x] **M1.3 — Engine: HTTP transport.** Function: `Target` → `*http.Request` → execute via `*http.Client` → `Result`. Handles errors (timeout, conn refused, DNS). Deps: M1.2. Done when: unit test against `httptest.Server` returns expected status/latency.
- [x] **M1.4 — Engine: worker pool.** N goroutines pulling tokens from a channel, calling transport, sending `Result`s out. Deps: M1.3. Done when: 100 workers fire 1000 requests against test server, `-race` clean.
- [x] **M1.5 — Engine: aggregator.** Goroutine consuming `Result`s, maintaining counters (sent, errors, status map) and one HDR histogram (service latency). Produces final `Summary`. Deps: M1.2. Done when: totals + percentiles match a known fixture.
- [x] **M1.6 — Engine: runner orchestration (basic).** Wire workers + aggregator. Honors `Plan.Requests` and `Plan.Concurrency`. No rate limit, no duration yet. Deps: M1.3, M1.4, M1.5. Done when: `Plan{Concurrency: 10, Requests: 100}` runs and returns valid `Summary`.
- [x] **M1.7 — CLI: skeleton.** Cobra root command, `--version`, `--help`. Deps: M1.1. Done when: `slam --version` and `slam --help` work.
- [x] **M1.8 — CLI: positional URL + implicit run.** First positional URL-shaped → implicit run. Heuristic: contains `://` or matches `host[:port][/path]`. Default scheme `http://`, default path `/`. Deps: M1.7. Done when: `slam localhost:3000/foo` produces correct `Target.URL`.
- [x] **M1.9 — CLI: minimum flags + text summary.** Flags `-c`, `-n`. Print final summary (totals, error rate, throughput, latency percentiles). Deps: M1.6, M1.8. Done when: `slam -n 100 -c 10 http://httpbin.org/get` prints a sensible summary.

## M2 — Complete v1

Goal: full v1 surface — rate limiting, ramp, indefinite mode, all HTTP knobs, polished CLI UX.

- [x] **M2.1 — Engine: duration-bounded run.** `Plan.Duration` support; runner stops after duration elapses. Deps: M1.6. Done when: `Duration: 5s` runs ~5s.
- [x] **M2.2 — Engine: indefinite run + cancel.** When `Duration=0 && Requests=0`, run until `ctx` cancels. Returns partial summary. Deps: M2.1. Done when: `ctx.Cancel()` mid-run produces summary of work done.
- [x] **M2.3 — Engine: scheduler with rate limit.** Token generator at `Plan.RPS`. Tokens carry `ScheduledAt`. Workers record both `ScheduledAt` and `SentAt`. Deps: M1.6. Done when: `RPS=1000` over 10s averages 1000 ± 1%.
- [x] **M2.4 — Engine: coordinated-omission correction.** Aggregator maintains second histogram: Response (`DoneAt - ScheduledAt`). Deps: M2.3. Done when: injecting 100ms transport stall produces visibly higher Response p99 vs Service p99.
- [x] **M2.5 — Engine: ramp-up.** Token rate ramps linearly 0 → RPS over `Plan.RampUp`. Deps: M2.3. Done when: first 5s of `RampUp=10s, RPS=1000` averages closer to 250 than 1000.
- [x] **M2.6 — Engine: HTTP knobs.** Full `Target` support — method, headers, body, query params. Plus `Plan.Timeout`, keep-alive toggle, HTTP/2 toggle. Deps: M1.3. Done when: each knob has a unit test verifying it's actually applied.
- [x] **M2.7 — CLI: full flag surface.** `-r`, `-t`, `--ramp`, `-H` (repeatable), `-d`, `--body-file`, `--query` (repeatable), `--method`, `--timeout`, `--no-keepalive`, `--http2`. Deps: M2.6, M1.9. Done when: each flag plumbed end-to-end with test.
- [x] **M2.8 — CLI: signal handling.** SIGINT/SIGTERM cancels `ctx`, runner returns partial summary, CLI prints it cleanly. Deps: M2.2, M1.9. Done when: ctrl-c during `slam url` produces a clean partial summary.
- [x] **M2.9 — CLI: preflight line.** Print `→ METHOD URL (workers, rate, ctrl-c to stop)` before run. Deps: M1.9. Done when: preflight matches actual flags.
- [x] **M2.10 — CLI: live TTY output.** Reporter impl: status line refreshed ~250ms with elapsed, sent, errors, current RPS, current p99. Auto-disabled when stdout isn't a TTY. Deps: M1.9. Done when: TTY shows refreshing line; piped output is plain.
- [x] **M2.11 — CLI: JSON output.** `-o json` prints final `Summary` as JSON, including compact-serialized histogram. Deps: M1.9. Done when: `slam -o json -n 10 url | jq` produces parseable JSON with all stats.

## M3 — Saturation testing

Goal: answer "how many requests per second can my server handle?" by stepping load up until the target degrades, then reporting the curve and the tipping point.

- [ ] **M3.1 — Engine: stepped scheduler.** Generalize M2.5's linear ramp into arbitrary `Plan.Stages []Stage{Duration, RPS}`. When `Stages` is set it overrides `RPS`/`RampUp`/`Duration`. Deps: M2.5. Done when: a 3-stage plan (10s @ 100 RPS, 10s @ 200, 10s @ 400) holds each step within ±5% of target.
- [ ] **M3.2 — Engine: per-stage aggregation.** Aggregator emits one `StageResult` per stage (counters + service & response histograms) alongside the overall `Summary`. Deps: M3.1, M1.5, M2.4. Done when: fixture with two stages produces two distinct histograms whose totals sum to the run total.
- [ ] **M3.3 — Engine: stop conditions.** `Plan.StopOn{MaxErrorRate, MaxP99, MinAchievedRatio}`. Runner watches live snapshots and cancels `ctx` when any rule trips, recording `StopReason` on the Summary. Deps: M3.2, M2.2. Done when: an httptest server returning 5xx after stage 2 stops the run with `StopReason=ErrorRate`.
- [ ] **M3.4 — CLI: `--scale-up` ergonomics.** `--scale-up START:PEAK:STEP --hold DURATION` expands to a `[]Stage`. `--stop-on-errors PCT --stop-on-p99 DURATION` configures stop rules. Deps: M3.1, M3.3, M2.7. Done when: `slam url --scale-up 50:1000:50 --hold 10s --stop-on-errors 5%` runs the right shape.
- [ ] **M3.5 — CLI: stepped table output + verdict.** Saturation runs print one row per stage (target RPS, achieved, p50, p99, error %), mark the stop-triggering row, and emit a one-line verdict on knee location (throughput plateau or p99 step-up). Deps: M3.2, M3.3, M2.11. Done when: a known-good httpbin-like server with deterministic 200ms p99 above 500 RPS produces a verdict line identifying the knee within ±1 stage.

## M4 — v1 Stretch (in if cheap)

- [ ] **M4.1 — CLI: YAML config file.** `-f config.yaml` loads run config; flags override file values. Stages are expressible. Deps: M2.7, M3.1. Done when: a YAML file with every option produces identical run to equivalent flags.
- [ ] **M4.2 — Engine: SQLite persistence.** Reporter impl writes run metadata + serialized histograms (per stage + overall) to `~/.slam/runs.db` using pure-Go driver. Deps: M2.4, M3.2. Done when: after a run, a row appears in `runs` table with retrievable histograms.

## M5 — Release & polish

- [ ] **M5.1 — Test suite consolidation.** Table-driven tests covering scheduler, aggregator, URL parsing, transport, signal handling, stages, stop conditions. Deps: alongside everything. Done when: `go test ./... -race` clean in CI.
- [ ] **M5.2 — CI pipeline.** GitHub Actions: golangci-lint, `go test -race`, build matrix (darwin/linux/windows × amd64/arm64). Deps: M5.1. Done when: PRs run all three checks.
- [ ] **M5.3 — GoReleaser config.** Cross-platform binaries + tarballs, signed + checksummed, attached to GitHub Release on tag push. Deps: M5.2. Done when: pushing `v0.1.0` produces a published release with binaries.
- [ ] **M5.4 — Homebrew tap.** Separate `homebrew-tap` repo, formula auto-updated by GoReleaser. Deps: M5.3. Done when: `brew install bradstell/tap/slam` works on macOS.
- [ ] **M5.5 — README + quickstart.** Install instructions, 3–5 example commands (incl. saturation), "what's next" pointer. Deps: M5.3. Done when: a stranger could install + run their first saturation test from the README alone.

## Out of scope (v2+)

- Pass/fail thresholds + non-zero exit codes for CI gating
- GUI (Wails) — moves to v1.x after v1 ships
- Distributed/multi-machine load (coordinator + workers)
- Multi-step scenarios (login → action → logout)
- Parameterized data from CSV
- Response body assertions
- Auth flows (OAuth, JWT refresh)
- HAR import
- Non-HTTP protocols (gRPC, WebSocket, raw TCP)
- Run comparison/diff
- Target resource monitoring

## Architectural notes

- `engine` is a pure library — CLI and (future) GUI are thin shells over it.
- HDR histogram (`HdrHistogram-go`) chosen for accuracy + mergeability (matters for per-stage aggregation in M3 and distributed mode in v2).
- Coordinated-omission correction baked in from M2.3+: store both `ScheduledAt` and `SentAt` per request, maintain two histograms (Service vs Response).
- Saturation testing (M3) builds on the rate scheduler from M2.5 — `Plan.Stages` is a generalization of `RampUp`. Per-stage histograms reuse the aggregator pattern.
- Concrete types in v1 (no premature interfaces). When scenarios and non-HTTP transports arrive in v2, promote `Target` → `RequestSource` and `*http.Client` → `Transport` interfaces.
- Pure-Go SQLite (`modernc.org/sqlite`) keeps cross-compilation simple — no CGO required.

# AI-Assisted Development Rules (Normative)

These instructions must be supplied to any AI coding agent before it modifies Avenlytic. Read `README.md` and `ARCHITECTURE.md` first.

## Mission

Build a lightweight, maintainable, cross-platform monitoring product. Preserve the architecture before optimizing for short-term implementation speed. Do not turn the project into a monolithic OS-specific collector or a heavy observability stack.

## Mandatory rules

1. **Do not break boundaries.** Core orchestration, models, rule evaluation, notification routing, and storage contracts must not import concrete OS adapters.
2. **Platform additions are additive.** To add FreeBSD or another OS, add a platform package, capability implementations, build-tag registration, tests, and build matrix entries. Do not add `if GOOS == ...` branches throughout core/server/UI.
3. **Use capability-specific interfaces.** Do not force every OS to implement every capability. Missing capability is explicitly unavailable.
4. **No fake metrics.** Never fill unsupported values with zero, guessed values, or values with different semantics. Include a warning/availability state.
5. **Keep common metrics truly common.** Document unit, type (gauge/counter), reset behavior, and sampling semantics. Namespace OS-specific metrics.
6. **Keep Agent lean.** Prefer standard library and native read-only APIs. Do not add a database, frontend runtime, or large server framework to the Agent. Avoid shelling out where a native API/file is available.
7. **Bound all work.** Every collector/network call has context cancellation and timeout. Use bounded worker pools and bounded retries. No unbounded goroutines or queues.
8. **No remote shell.** Never add remote command execution, arbitrary script execution, file browsing, or privileged actions to the monitoring protocol without a separate explicit security design and audit model.
9. **Notification providers are plugins by interface.** Implement `notify.Channel`; register it. Never add provider-specific logic to rule evaluation. Redact secrets in read APIs and logs.
10. **Alert state and delivery state are different.** An alert may be firing while one channel succeeds and another fails. Persist delivery attempts independently. Recovery is a distinct transition.
11. **Storage is replaceable.** Business logic must not depend on JSON file layout. Add repository/store interfaces before adding PostgreSQL or another backend.
12. **Frontend stays independent.** It uses public API contracts only. No runtime CDN dependency. Keep UI assets embedded in the server, never in the Agent.
13. **No speculative dependencies.** Before adding a library, explain why standard library/native implementation is insufficient, assess license and maintenance, and keep it out of the Agent if possible.
14. **Security by default.** Do not log secrets. Validate input, cap request bodies, use HTTPS in deployment, protect state files, and consider SSRF when accepting webhook URLs.
15. **Do not claim cross-platform support based on cross-compilation alone.** Add native integration tests on the actual OS. If not tested, mark it as compile-only/placeholder.

## Required workflow for every task

1. State the intended change and affected packages.
2. Read the relevant interfaces and tests before editing.
3. Propose any contract/API change before implementing it.
4. Implement the smallest coherent change without unrelated refactoring.
5. Add unit tests for success, unsupported capability, malformed input, and failure behavior as relevant.
6. Run `gofmt -w` on changed Go files.
7. Run `go test ./...` and `go vet ./...`.
8. Run relevant cross-compilation targets. State clearly what was and was not tested on real operating systems.
9. Update `README.md`, `ARCHITECTURE.md`, API docs, and examples when behavior/contracts change.
10. Report exact changed files, commands run, test outcomes, and remaining limitations. Never claim a test passed unless it was run.

## Prohibited shortcuts

- OS checks scattered across the core.
- `unsafe` or cgo without a documented need and platform test plan.
- Go `plugin` for cross-platform plugin loading.
- Blocking collector calls with no timeout.
- Treating missing metrics as healthy/zero.
- Storing notification secrets in frontend local storage.
- Returning SMTP passwords, bot tokens, or webhook URLs from API list endpoints.
- Infinite notification retry loops.
- Changing a protocol field's meaning without versioning.
- Adding a new platform by copying Linux code and renaming it.

## AI task prompt template

> You are contributing to Avenlytic. First read `README.md`, `ARCHITECTURE.md`, and `docs/DEVELOPMENT_RULES.md`. Follow all normative rules. Before coding, identify affected packages and interfaces. Preserve platform-neutral core boundaries; implement OS-specific behavior only in `internal/platform/<goos>`. Use capability-specific contracts, explicit unsupported states, bounded contexts/timeouts, and tests. Do not add unrelated dependencies or refactor unrelated modules. Run formatting, tests, vet, and relevant cross-builds. Update documentation. In your final report list changed files, verification commands/results, and unimplemented limitations. Do not claim native platform support unless tested on that OS.

## Web UI and authentication constraints

- Production UI changes belong in `web-ui/` (Vue 3 + TypeScript + Vite). Do not add new product pages only to the fallback `cmd/server/web` implementation; keep fallback changes limited to first-run/login/basic operational access and maintain parity where practical.
- Build the Vue UI before compiling a production Server binary. Docker and CI workflows must preserve this order so `go:embed` captures the Vue build output.
- Browser admin credentials must never be stored in LocalStorage or SessionStorage. Use HttpOnly SameSite cookies. Admin API routes must require an authenticated session.
- Agent bearer credentials are only for ingest and must never authorize admin APIs or be returned by a settings endpoint.
- Password storage must use a salted, slow password hash. Never log password values or session cookies.
- UI must expose supported operational settings, validation errors and channel test results. Clearly label settings that are install-time defaults rather than remotely hot-applied configuration.

# Avenlytic Architecture Contract

This document defines boundaries that all future changes must preserve.

## 1. System topology

```text
Linux / Windows / macOS Agent
  ├─ Platform Adapter (OS-specific)
  ├─ Collectors (capability-specific)
  ├─ Scheduler / lifecycle
  ├─ Metric normalization (core model)
  └─ Outbound transport (versioned protocol)
             │ HTTPS / JSON
             ▼
Central Go Server
  ├─ API and authentication
  ├─ Host inventory / heartbeat
  ├─ Rule evaluator and alert lifecycle
  ├─ Notification dispatcher / channel registry
  ├─ Storage abstraction
  └─ Embedded Web Console
```

The agent is a pull-free, outbound-only telemetry client. The server never needs to initiate a connection to a monitored host for normal collection.

## 2. Dependency direction

Allowed dependency direction:

```text
cmd -> core orchestration -> contracts/models
platform adapters -> contracts + core models
collectors -> contracts + core models
server -> core models + rules + notification + storage abstractions
web -> public HTTP API only
```

Forbidden:

- `core` importing `platform/linux`, `platform/windows`, `platform/darwin`, `syscall`, `golang.org/x/sys/unix`, WMI, Mach, `/proc`, or `/sys` implementation details.
- Collectors reading OS-specific files or invoking OS-specific APIs directly.
- Platform adapters importing server, UI, notification delivery, or database packages.
- Web code depending on internal Go package structure.
- A platform-specific field being added to a common model without a portable semantic meaning.

## 3. Platform adapter contract

`internal/core/ports` is the only boundary between platform-independent orchestration and OS-specific collection. The base provider may expose platform identity and a normalized snapshot for compatibility. New collector work must use capability-specific optional interfaces (CPU, memory, disk, network, process, service, sensors) rather than adding mandatory methods to a monolithic interface.

Rules:

1. Every capability is independently discoverable.
2. A platform implements only capabilities it can genuinely support.
3. Unsupported capability means absent/unavailable plus a diagnostic; never a fabricated zero.
4. Permissions and privilege requirements are metadata, not assumptions.
5. Platform-specific metrics use namespaced identifiers (for example `linux.memory.slab`); common metrics must have identical units and semantics on all platforms.
6. A new OS may add `internal/platform/<goos>`, build-tag selection, tests, and build-matrix entries. It must not require changes to core orchestration, rule evaluation, notification logic, or web UI.
7. Never use Go's `plugin` package as the cross-platform extension mechanism. Built-in adapters use compile-time composition/build tags. A future out-of-process or WASM plugin protocol must be versioned and isolated.

## 4. Collector contract

A collector has a stable ID, human-readable metadata, supported capability requirements, default interval, timeout, and a collection function. It must:

- be safe to run repeatedly;
- respect context cancellation and configured timeouts;
- avoid unbounded goroutines, subprocesses, or memory growth;
- avoid blocking other collectors;
- emit typed normalized metrics with units and timestamps;
- distinguish unsupported, permission denied, transient error, and empty-but-valid results;
- avoid collecting secrets or high-cardinality labels by default.

Collection scheduling belongs to core orchestration. Platform adapters provide facts; they do not own global scheduling or network transport.

## 5. Metric and protocol contract

- Protocol is explicitly versioned (`v1`). Breaking payload changes require a new version or a documented backward-compatible migration.
- Metric names, units, labels, and aggregation semantics must be documented.
- Counter metrics (network/disk bytes) are monotonic where the OS exposes them; rates are derived from two samples and must handle counter reset/device removal.
- Gauges (CPU%, memory%, load) are point-in-time values.
- Missing data is not zero.
- Agent identity is stable and distinct from display hostname.
- Server validates payload size, protocol version, identity, and timestamp.

## 6. Alert lifecycle

```text
normal -> pending (condition true) -> firing -> resolved
                               └-> normal (condition cleared before duration)
```

- Rule evaluation is independent of notification provider implementations.
- A rule contains metric, operator, threshold, duration, severity, channel references, enabled state, and recovery policy.
- Missing metric values do not match.
- Duration debounces transient conditions.
- Agent-offline is a server-side heartbeat rule; it must not depend on the dead agent sending a final message.
- Every transition is an event. Notification delivery outcome is recorded separately from alert state.
- Acknowledge must not silently resolve an alert.

## 7. Notification channel contract

A channel implements `notify.Channel` with `Type()` and `Send(context.Context, Config, Message) error`.

- Each provider owns its payload formatting, signing, authentication, and response validation.
- Secrets are never returned by read APIs or written to logs.
- Each send has a deadline; retries are bounded.
- Delivery status, attempt count, and error are recorded separately from the alert event.
- New channel addition must not add provider-specific branches to the rule evaluator.
- Webhook destinations are untrusted configuration; production deployments should apply egress allowlists and SSRF protections.

## 8. Storage contract

Business services must not depend on JSON serialization or a concrete database driver. Storage operations should be expressed through repository/store interfaces. The current server depends on `storage.Repository` rather than a concrete database driver. SQLite is the default single-instance backend; PostgreSQL or a time-series database can be added for multi-instance or high-volume deployments.

## 9. Frontend contract

- Frontend communicates only through documented versioned HTTP APIs.
- Static assets are built independently and embedded into the Go server; the Agent must not include frontend assets.
- UI must reflect capability availability rather than hard-code OS-specific assumptions.
- No CDN dependency at runtime.
- Secrets are write-only: forms may accept them, but list APIs return redacted channel metadata.

## 10. Security and operations

- Default to least privilege. Agent collection is read-only.
- Never add remote command execution as a monitoring convenience.
- Use HTTPS, unique credentials, rate limits, request limits, and audit logs in production.
- Avoid logging bearer tokens, passwords, signed URLs, or complete webhook URLs.
- Every network request has a timeout; retry loops use bounded exponential backoff.
- State files and backups containing channel settings must be protected.

## 11. Definition of done

A contribution is not complete until:

1. `gofmt` has been run.
2. `go test ./...` passes.
3. `go vet ./...` passes where applicable.
4. Relevant platform cross-builds pass.
5. New behavior has tests, including unsupported/error/permission paths.
6. Public API and metric documentation are updated.
7. README and AI development rules remain consistent with actual implementation.
8. No platform-specific dependency has leaked into core packages.

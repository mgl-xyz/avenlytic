# Roadmap

## Implemented in this source release

- Linux host snapshot collection (CPU, memory, load, uptime, filesystem capacity, block I/O, network counters, process count and top process sample).
- Agent identity, outbound ingest, bearer-token protection, retry/backoff.
- Host inventory, heartbeat state, threshold rules, duration debounce, recovery lifecycle.
- SMTP, DingTalk, Feishu, WeCom, Slack, Telegram and generic webhook channels.
- Bounded notification retry and delivery records.
- Embedded HTML/CSS/JavaScript console for overview, hosts/details, alerts, rules, channels, delivery history and settings.
- SQLite persistence with transactional updates and legacy JSON import.

## Next engineering milestones

### P1 — Production storage and identity

- PostgreSQL implementation behind storage repository interfaces.
- Schema migrations, indexes, retention, aggregation, backup/restore.
- Per-agent credentials, enrollment/revocation, credential rotation.
- RBAC, audit trail, admin authentication, rate limits.
- Durable transactional notification outbox and background worker.

### P2 — Linux operations coverage

- systemd unit status and selected service monitoring.
- Configurable process-presence rules and process restart detection (read-only observation first).
- Failed SSH/login attempt monitoring from supported Linux authentication logs, with configurable time windows and thresholds.
- Available package update counts for supported Linux distributions.
- Installed-package vulnerability matching against a maintained advisory feed, including feed freshness and unsupported-package reporting.
- TCP/HTTP/HTTPS/DNS checks and certificate expiry checks.
- SMART/temperature/GPU sensors as optional collectors with explicit permissions.
- Per-interface throughput rates and disk I/O rates derived from counter deltas.

### P3 — Native platform adapters

- Windows: PDH performance counters, GlobalMemoryStatusEx, GetIfTable2, process APIs, Service Control Manager.
- macOS: Mach host statistics, sysctl, libproc, filesystem/network APIs, launchd status where permitted.
- FreeBSD: sysctl, devstat, procstat and service-manager adapter.
- Real-OS integration tests and signed release artifacts.

### P4 — Fleet and UX

- Historical metrics and trend charts.
- Host groups, tags, labels, maintenance windows and notification schedules.
- Alert silencing, deduplication, escalation and notification templates.
- Agent remote configuration with signed configuration revisions.
- Signed self-update and rollback.

## Non-goals unless separately designed

- Remote shell or arbitrary command execution.
- General-purpose configuration management.
- Heavy distributed tracing/log indexing stack bundled into the Agent.


## Security & reliability scope (next major milestone)

The current 0.4.x agent is a resource monitor, not yet a host-security product. Do not market the existing release as intrusion detection or vulnerability scanning. Implement these as explicit, permission-aware collectors:

### Host security telemetry

- Authentication events: failed/successful SSH and local logins, source address, account, timestamp, and event source. Support journald plus distro-specific auth logs (`/var/log/auth.log`, `/var/log/secure`); persist a cursor so restarts do not replay the entire log. Never collect passwords, private keys, or full command lines by default.
- Detection rules: repeated failures by source/account, success after repeated failures, root SSH login, unexpected new privileged account, sudo authentication failures, and SSH configuration drift. Use configurable thresholds, allowlists, suppression windows, and confidence/severity labels.
- Network exposure: listening TCP/UDP ports, service/process association, firewall state where readable, and optional external reachability checks. Local listening sockets alone must not be described as an Internet-exposed service.
- System maintenance: OS release, package manager, pending updates, reboot-required marker, last successful update time where available, and failed systemd units. Never auto-install updates in the first iteration.

### Vulnerability intelligence

- Build an advisory ingestion service for OSV and distro-native security feeds (for example Debian Security Tracker, Ubuntu USN, and Red Hat OVAL/errata). Store source, advisory ID, published/modified time, affected package ranges, fixed versions, and references.
- Agent reports package inventory (name, epoch/version/release, architecture, source package, distro/release). Prefer native package database APIs/commands and make collection optional because inventory can expose software footprint.
- Match with distro-aware version semantics and vendor backports. A naive `version < fixed_version` comparison is not valid across RPM/Debian epochs, revisions, and backported patches.
- Findings must include evidence, affected package, installed version, fixed version if known, advisory source, confidence, and last-checked time. Separate `potentially affected`, `vendor fixed/backported`, `not affected`, and `unknown`; never claim a host is compromised from a CVE match.
- Add feed freshness monitoring, cache/signature validation where supported, rate limits, and an explicit stale-feed warning. No remote arbitrary command execution.

### Agent identity and configuration

- Replace the current shared fleet-wide bearer token with enrollment tokens and per-agent credentials. Support one-time enrollment, per-agent rotation/revocation, last-used time, and an audit record. Keep secrets hashed at rest where verification permits; show plaintext only once.
- Signed, versioned remote configuration with polling/long-polling and explicit acknowledgement. Until that exists, the collection interval is a local Agent CLI setting, not a live server-side setting.
- Add an explicit Agent inventory page showing ID, credential status, Agent version, last heartbeat, reported interval, capabilities, and configuration revision.

### Product acceptance criteria

- Every visible control has a working API and persisted behavior, or is clearly marked as planned/unsupported.
- Distinguish browser dashboard refresh interval from Agent collection interval and offline timeout.
- Security events are deduplicated, timestamped, searchable, retained according to policy, and linked to a host and evidence source.
- Error notifications preserve the complete server response, remain visible on hover/focus, can be dismissed/copied, and do not hide diagnostic details.
- Add integration tests for log cursor behavior, event deduplication, distro version comparison, credential rotation/revocation, and configuration acknowledgement.

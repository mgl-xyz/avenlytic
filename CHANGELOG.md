# Changelog

## Unreleased

- Extend Linux host collection with CPU model/vendor/frequency and hwmon/thermal-zone temperature and fan sensors.
- Add explicit `--sys-root` support to container Agent deployments and surface hardware sensor data in host details.

## 0.4.1 — Developer workflow and console UX

- Add `make dev` to launch the Go API server and Vue/Vite development server together; stop both when either process exits.
- Refresh the console visual system with a restrained light theme, softer surfaces, clearer typography, and subtle interaction transitions.
- Use a conventional system font stack with common Chinese fallbacks (PingFang SC, Microsoft YaHei, Noto Sans CJK SC, Source Han Sans SC).
- Make dashboard auto-refresh configurable (5s, 10s, 15s, 30s, 1m, or disabled); persist the choice in this browser.
- Clarify Agent bearer-token usage and add show/generate/copy affordances. Copy the newly generated token before saving; the server intentionally does not return the stored secret.
- Clarify that the Agent collection interval is a CLI/deployment default and does not hot-update already-running Agents.
- Separate edit/delete actions visually; make notifications longer-lived, hover-pausable, dismissible, and easier to read.
- Expand the roadmap with a concrete, evidence-based host security and vulnerability-intelligence design.

## Important limitations

- This release does not yet implement SSH/authentication event collection, intrusion detection, package inventory, CVE matching, or OS update inventory. These are explicitly documented as next milestones.
- The Agent credential remains a shared fleet-wide token stored in the local JSON state file. Per-Agent enrollment and revocation are still required for production-grade credential management.
- Server-side remote configuration is not implemented; changing the Agent interval affects newly launched Agents using the generated command only.

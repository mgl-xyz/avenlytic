# UI previews

These PNGs are static, illustrative UI previews rendered from the Avenlytic v0.3 visual design with synthetic sample data. They are included to document the intended appearance of the first-run setup, dashboard, alert rules, notification channels and system settings. They are not screenshots of a live production deployment.

The production console is implemented in `web-ui/` with Vue 3 + TypeScript + Vite. Docker and GitHub Actions build that Vue app and embed the resulting assets into the Go Server. The checked-in `cmd/server/web/` assets are a no-Node fallback console for direct Go development.

- `01-first-run-setup.png` — initial administrator setup
- `02-dashboard.png` — infrastructure overview
- `03-alert-rules.png` — alert rule management
- `04-notification-channels.png` — channel configuration and delivery history
- `05-system-settings.png` — Agent token, offline timeout and account settings

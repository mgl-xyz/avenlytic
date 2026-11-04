# Notification design

## Pipeline

`Metric -> Rule evaluation -> Alert transition -> Notification dispatch -> Channel adapter -> Delivery record`

Alert events and delivery attempts are independent records. For example, an alert may remain `firing` even if DingTalk is unavailable; the failed DingTalk delivery must not resolve or delete the alert.

## Rule behavior

A rule selects a metric, operator, threshold, duration, severity, one or more channel IDs, and a recovery-notification preference. Duration is a debounce window. The agent-offline rule is evaluated by the server using last-seen timestamps.

## Built-in channel types

| Type | Required settings | Notes |
|---|---|---|
| `email` | smtp_host, smtp_port, from, to | Optional username/password; TLS configuration depends on SMTP provider |
| `dingtalk` | webhook_url | Optional secret enables signed URL parameters |
| `feishu` | webhook_url | Optional secret enables signature fields |
| `wecom` | webhook_url | Markdown group bot |
| `slack` | webhook_url | Incoming webhook |
| `telegram` | bot_token, chat_id | Telegram Bot API |
| `webhook` | url | Generic JSON message body |

## Add a channel

1. Create a type-specific struct in `internal/notify`.
2. Implement `Type()` and `Send(ctx, config, message)`.
3. Validate required settings and provider response status.
4. Register it in `notify.NewRegistry()`.
5. Add a channel type to the API allowlist and UI form fields.
6. Add unit tests using `httptest` or a fake SMTP server; never call a real production webhook in unit tests.
7. Ensure secrets are not returned by list APIs and are not logged.

## Delivery behavior in this release

The dispatcher makes up to three bounded attempts with short backoff and records status, attempts, and final error. Delivery is dispatched asynchronously from alert evaluation. This is not yet a durable queue: a server crash during an in-flight send can lose that send. A production queue/outbox worker with transactional persistence and configurable retry policy is a planned upgrade.

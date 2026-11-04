# HTTP API v1

Browser administration uses an HttpOnly session cookie. API clients can obtain an HS256 JWT and pass it using the standard Bearer authentication scheme. Agent credentials are separate and only authorize ingest endpoints. `/healthz` and Agent ingest do not require the optional UI entry link. When that gate is enabled, requests to the UI and admin API must first visit the configured private path; other UI/admin paths return 404.

| Method | Path | Purpose |
|---|---|---|
| GET | `/healthz` | Health and protocol version |
| GET | `/api/v1/auth/status` | First-run setup status |
| POST | `/api/v1/auth/setup` | Create the first administrator and browser session |
| POST | `/api/v1/auth/login` | Create a browser session |
| POST | `/api/v1/auth/token` | Issue a 12-hour HS256 JWT for an API client |
| POST | `/api/v1/auth/logout` | End the browser session |
| GET | `/api/v1/auth/me` | Current authenticated administrator |
| GET | `/api/v1/events` | Authenticated Server-Sent Events stream; sends an initial monitoring snapshot and pushes updates |
| POST | `/api/v1/ingest` | Agent snapshot ingest |
| GET | `/api/v1/hosts` | Host snapshots and online status |
| GET | `/api/v1/summary` | Fleet summary |
| GET/POST | `/api/v1/rules` | List/create/update rules |
| DELETE | `/api/v1/rules/{id}` | Delete rule |
| GET/POST | `/api/v1/channels` | List channels with URLs and non-credential settings / create or update channel |
| GET | `/api/v1/channels/{id}` | Get channel details with URLs and non-credential settings for editing |
| POST | `/api/v1/channels/test` | Test an unsaved channel configuration |
| POST | `/api/v1/channels/{id}/test` | Test a saved channel without returning its secrets |
| DELETE | `/api/v1/channels/{id}` | Delete channel |
| GET | `/api/v1/alerts` | Alert event history |
| POST | `/api/v1/alerts/{id}/ack` | Acknowledge an active alert |
| GET | `/api/v1/deliveries` | Notification delivery history |

The browser UI uses same-origin SSE to receive the initial monitoring snapshot and live host, summary, and alert updates without refetching those endpoints. The configured polling interval is used only while the event stream is disconnected. Browser routes are `/login` for setup/sign-in and `/`, `/hosts`, `/containers`, `/alerts`, `/rules`, `/channels`, and `/settings` for the authenticated application; navigation supports direct links and browser back/forward.

## JWT for API clients

Request a token with administrator credentials:

```http
POST /api/v1/auth/token
Content-Type: application/json

{"username":"operator","password":"your-password"}
```

The response contains `access_token`, `token_type`, and `expires_at`. Include the token in the request's Authorization header with the Bearer scheme to access admin endpoints. The signing key is generated randomly and stored with the server state; changing the administrator password rotates it and invalidates outstanding JWTs. Browser login continues to use the HttpOnly cookie session.

## Optional UI entry link

The private entry path is configured in System Settings. It must contain 3-128 URL-safe characters. Visiting `/<private-path>` sets a temporary HttpOnly access cookie and redirects to `/`; UI and admin API requests without that cookie return 404. The path hash is stored in the server SQLite database, and the original path is not returned by the settings API. The generated link is shown in System Settings after generation and can be copied there; save it before closing the page because it cannot be retrieved later. Short paths are easier to guess, so use a randomly generated path over HTTPS. This gate is an extra layer and does not replace administrator authentication.

## Ingest payload

```json
{
  "protocol": "v1",
  "agent_id": "stable-agent-id",
  "hostname": "web-01",
  "os": "linux",
  "arch": "amd64",
  "platform": "Ubuntu 24.04",
  "collected_at": "2026-11-02T10:00:00Z",
  "cpu": {"usage_percent": 12.5, "cores": 8},
  "memory": {"total_bytes": 17179869184, "available_bytes": 8589934592, "used_bytes": 8589934592, "usage_percent": 50},
  "capabilities": ["cpu", "memory", "disk_capacity"]
}
```

The actual snapshot may include load, uptime, network counters, disk usage, disk I/O, process count, process samples, runtime data, optional Docker inventory, and warnings. Missing metrics are omitted rather than populated with invented zero values.

## Optional Docker inventory

Linux Agents collect Docker data only when started with `--docker-socket <path>`. The option is disabled by default. The Agent performs Docker Engine `GET` requests for `/version`, `/containers/json?all=1&limit=100`, and each running container's `/containers/{id}/stats?stream=false`. It reports container names, images, state/status, creation time, ports, and available CPU/memory usage in the optional `docker` snapshot object. The list is capped at 100 containers; CPU and memory values are omitted if unavailable. Failed inventory collection is reported using `docker.available: false` and `docker.error`; unsupported or older Agents omit the object. The authenticated `/api/v1/hosts` and SSE snapshots expose this data to the read-only `/containers` console page, which lets the operator select one Docker-enabled host at a time and inspect only that host's containers.

The Agent issues only read requests, but access to the Docker Engine socket itself normally grants full control of the Docker daemon. Do not treat a read-only filesystem mount as an API permission boundary. For least privilege, point the Agent at a socket proxy that restricts methods and endpoints to the required read operations.

Linux Agents running in containers can use `--host-root <mount-path>` to collect host metrics instead of the container's own `/proc` and root filesystem. Mount the host root read-only at that path and mount host `/proc` read-only below it as `<mount-path>/proc`; namespace-dependent mount and network views are read through host `/proc/1`. The optional `--sys-root` path selects mounted host sysfs for CPU frequency and hwmon/thermal sensors. These mounts are read-only and do not require privileged mode or host namespaces, but host-root access grants read visibility to files available to the Agent UID. Hardware sensors and frequency data are emitted only when supported by the host driver. Host login-failure logs, package inventory, update checks, and systemd journal collection are not supported in host-root container mode.

## Rule payload

```json
{
  "id": "optional-on-create",
  "name": "Memory usage high",
  "enabled": true,
  "metric": "memory.usage_percent",
  "operator": ">",
  "threshold": 85,
  "duration_seconds": 60,
  "severity": "warning",
  "channel_ids": ["channel-id"],
  "notify_on_recovery": true
}
```

`agent.online` is evaluated by the server's heartbeat monitor. Its threshold/operator are currently accepted for schema uniformity but are not used in the heartbeat comparison; `duration_seconds` is the additional debounce period after the server's 45-second heartbeat timeout.

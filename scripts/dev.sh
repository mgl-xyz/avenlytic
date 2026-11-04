#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
for cmd in go npm; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "缺少依赖：$cmd" >&2; exit 1; }
done
if [[ ! -d web-ui/node_modules ]]; then
  echo "[web] 安装前端依赖…"
  (cd web-ui && npm install --no-audit --no-fund)
fi
server_pid=""; web_pid=""
cleanup() {
  trap - INT TERM EXIT
  [[ -n "$web_pid" ]] && kill "$web_pid" 2>/dev/null || true
  [[ -n "$server_pid" ]] && kill "$server_pid" 2>/dev/null || true
  [[ -n "$web_pid" ]] && wait "$web_pid" 2>/dev/null || true
  [[ -n "$server_pid" ]] && wait "$server_pid" 2>/dev/null || true
}
trap cleanup INT TERM EXIT

echo "[server] http://127.0.0.1:9080"
go run ./cmd/server &
server_pid=$!
# Wait for the backend listener so the first Vite proxy requests do not race startup.
for _ in {1..60}; do
  if ! kill -0 "$server_pid" 2>/dev/null; then echo "[server] 启动失败" >&2; exit 1; fi
  if (echo > /dev/tcp/127.0.0.1/9080) >/dev/null 2>&1; then break; fi
  sleep 0.5
done
if ! (echo > /dev/tcp/127.0.0.1/9080) >/dev/null 2>&1; then echo "[server] 等待 9080 端口超时" >&2; exit 1; fi
echo "[web]    http://127.0.0.1:5173"
(cd web-ui && npm run dev -- --host 0.0.0.0) &
web_pid=$!
# Stop both processes if either side exits unexpectedly.
set +e
wait -n "$server_pid" "$web_pid"
status=$?
set -e
if kill -0 "$server_pid" 2>/dev/null || kill -0 "$web_pid" 2>/dev/null; then
  echo "一个开发服务已退出，正在停止另一个服务…"
fi
exit "$status"

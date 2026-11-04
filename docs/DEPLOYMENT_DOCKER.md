# Docker 部署

## 启动中心 Server

```bash
cp .env.example .env
docker compose up -d
```

默认只启动中心 Server，不会挂载宿主机目录。浏览器访问 `http://HOST:9080`，首次进入创建管理员，再到「系统设置」生成 Agent Token。Server 数据保存在 Docker named volume `avenlytic-data`。

## 在 Docker 中采集宿主 Linux

容器中的 `/proc`、`/etc/os-release`、挂载表和根文件系统默认属于容器，不能代表宿主。仓库提供独立 Agent 镜像和 `host-agent` Compose profile；该模式面向原生 Linux Docker 主机，通过只读挂载宿主根目录、`/proc` 和 `/sys`，读取宿主 CPU、内存、负载、网络、进程、磁盘、CPU 型号/频率和硬件传感器。bind mount 的来源是 Docker Engine 所在主机；远程 Docker context 会监控远端 daemon 主机，而不是运行 Compose 命令的客户端。Docker Desktop 不提供通用的原生 Windows/macOS `/proc` 挂载，因此不能用此 profile 监控桌面宿主。

先将控制台生成的 Agent Token 写入 `.env` 的 `AGENT_TOKEN`，然后启动：

```bash
docker compose --profile host-agent up -d --build
docker compose logs -f agent
```

`.env` 中的 Agent Token 必须与控制台当前 Token 一致；轮换后更新 `.env` 并重建/重启 Agent：

```bash
docker compose --profile host-agent up -d --force-recreate agent
```

Agent 容器以非 root 用户运行，丢弃全部 Linux capabilities，启用 `no-new-privileges` 和只读容器根文件系统；**不需要 `privileged`、`network_mode: host` 或 `pid: host`**。宿主 `/`、`/proc` 和 `/sys` 均以只读方式挂载到容器的 `/host`、`/host/proc` 和 `/host-sys`。`/sys` 用于读取 CPU 型号/频率、hwmon 温度和风扇传感器。宿主挂载点通过 `/host` 的递归 bind 可见，并使用 `rslave` 接收后续挂载变化；依赖宿主 PID/网络命名空间的挂载表和网络接口读取宿主 `/proc/1` 视图。Docker 的递归只读子挂载需要 Linux Kernel 5.12 或更新版本；旧内核上根目录下的子挂载可能仍为可写，因此不要在旧内核上启用该 profile，建议改为在宿主直接运行 Agent。若宿主启用了 `hidepid`、SELinux/AppArmor 限制、rootless Docker 或文件系统挂载隔离，部分进程/磁盘/硬件数据也可能不可见。

宿主 Agent 容器读取宿主根目录中的文件，因此这会扩大该容器的**只读文件访问范围**；如不接受此信任边界，推荐直接在宿主运行 Agent 二进制/systemd，而不是容器化 Agent。只读挂载不等于文件不可读，宿主敏感文件若本身对该 UID 可读仍可被读取。温度/风扇依赖内核 `hwmon` 或 thermal-zone 驱动，传感器不存在或权限不足时会省略并在快照 warning 中说明；CPU 频率也依赖 cpufreq 驱动。Docker 容器各自的 CPU/内存统计仍通过 Docker API 页面采集。此模式不查询宿主登录失败日志、不执行宿主的软件包管理器，也不扫描宿主软件包数据库；登录失败监控、系统更新和软件包清单请在宿主直接运行 Agent。当前也不启用宿主 systemd journal 查询。

Agent 默认不访问 Docker Engine，因此不会自动列出容器。容器清单功能仍需显式配置 `--docker-socket`。Docker Socket 通常等价于 Docker daemon 的完全控制权，即使 Agent 只发 GET 请求或 Socket 以只读文件方式挂载也不构成 API 权限限制。请优先使用仅允许 `/version`、`/containers/json` 和 `/containers/{id}/stats` 所需 GET 请求的 Socket 代理；不要为了省事把 `/var/run/docker.sock` 直接挂给 Agent。仓库提供可选 Compose override，将宿主的受限 Unix Socket 只读挂给 Agent：

```bash
# .env: DOCKER_SOCKET_PROXY_PATH=/run/docker-ro.sock
docker compose -f docker-compose.yml -f docker-compose.agent-docker.yml \
  --profile host-agent up -d --build
```

不要把真实 Docker daemon Socket 填入 `DOCKER_SOCKET_PROXY_PATH`。代理 Socket 的宿主文件权限必须允许容器内 UID 10001 连接；不需要把 Agent 加入 `docker` 组或额外授予 Linux capabilities。Socket 的 bind mount 即使标为只读也不会限制 API 写操作，必须由代理自身拒绝非必要方法/端点。

如需绕过 Docker Compose，Agent 运行参数也支持直接指定宿主路径：

```bash
avenlytic-agent --server https://monitor.example.net \
  --token '与中心端一致的Token' --host-root /host
```

这要求宿主根目录以只读方式挂载到 `/host`，并把宿主 `/proc` 以只读方式挂载到 `/host/proc`。`--sys-root` 可指向宿主 sysfs 挂载点；路径省略时，Agent 使用 `--host-root` 下的 `sys`。若不需要硬件指标，可不挂载 sysfs 并省略该参数；Agent 会通过 warning 报告缺失，不会伪造为 0。Agent 必须能读取宿主 `/etc/hostname` 或 `/proc/sys/kernel/hostname`；也可显式传 `--name`。Linux Agent 的这些选项不会自动授予文件权限或绕过 LSM 策略。

## 持久化和备份

升级镜像不会删除 named volume。备份建议：

```bash
docker compose stop server
docker run --rm -v avenlytic_avenlytic-data:/data -v "$PWD":/backup alpine \
  tar czf /backup/avenlytic-data-$(date +%F).tgz -C /data .
docker compose start server
```

停止服务后再复制数据库，避免 SQLite WAL 主库与日志文件在备份过程中状态不一致。

## 本地构建

```bash
docker compose up -d --build
```

Dockerfile 使用 Node.js 26 LTS 构建 Vue 3 前端，再使用 Go 1.27 编译嵌入前端资源的单一 Server 二进制。最终运行镜像不包含 Node.js 或 Go 工具链。
`Dockerfile.agent` 单独构建不含 Go 工具链的轻量 Linux Agent 镜像；通过 `--profile host-agent` 启用时，`--build` 会在本机构建该镜像；不带 `--build` 时 Compose 会优先复用/拉取配置的 Agent 镜像。

## 发布镜像

GitHub Actions 在推送 `v*` Tag 时构建并发布：

- `ghcr.io/OWNER/REPOSITORY:latest`
- `ghcr.io/OWNER/REPOSITORY:vX.Y.Z`
- `ghcr.io/OWNER/REPOSITORY-agent:latest`
- `ghcr.io/OWNER/REPOSITORY-agent:vX.Y.Z`

首次使用 GHCR 时，请确保仓库允许 Actions 发布 Packages。`docker-compose.yml` 默认使用 `ghcr.io/avenlytic/avenlytic:latest` 和 `ghcr.io/avenlytic/avenlytic-agent:latest`，若仓库地址不同，请在 `.env` 中设置 `GITHUB_REPOSITORY=OWNER/REPOSITORY`。本地构建可使用 `docker compose --profile host-agent up -d --build`。

## 生产部署安全

- 将 9080 放在 HTTPS 反向代理之后，不要直接暴露到不可信网络。
- 首次设置后立即创建高强度管理员密码和 Agent Token。
- `/data/avenlytic.db` 是 SQLite 数据库，保存管理员密码哈希、Agent Token、通知通道密钥和告警数据；限制卷访问权限并加密备份。首次启动会自动导入卷中旧版 `/data/avenlytic-state.json`，并保留原 JSON 文件不变。
- 当前会话 Cookie 在直连 TLS 时设置 Secure；TLS 终止代理部署时，应使用可信反向代理并确保外部访问为 HTTPS。
- SQLite 使用 WAL 和事务，面向单实例部署；多实例/高吞吐场景仍需要 PostgreSQL 等服务端数据库。

## Docker Hub

To publish Docker Hub images alongside GHCR, configure the GitHub repository variable `DOCKERHUB_USERNAME` and secret `DOCKERHUB_TOKEN` (Docker Hub Access Token). The release workflow publishes `docker.io/<username>/avenlytic` for `linux/amd64` and `linux/arm64`, tagged with the release version and `latest`. Without the username variable, this optional job is skipped.

To pull from Docker Hub, set `AVENLYTIC_IMAGE=docker.io/<username>/avenlytic:latest` and `AVENLYTIC_AGENT_IMAGE=docker.io/<username>/avenlytic-agent:latest` in `.env`, then run `docker compose pull && docker compose up -d`. To pull and start the optional Agent, include its profile: `docker compose --profile host-agent pull agent && docker compose --profile host-agent up -d agent`.

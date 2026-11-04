# Avenlytic AI 开发入口规范

将以下要求作为每次 AI 代码任务的前置上下文：

1. 开始前必须完整阅读 `ARCHITECTURE.md`、`docs/DEVELOPMENT_RULES.md`、`docs/API.md`，涉及告警/通知时还必须阅读 `docs/NOTIFICATIONS.md`；涉及部署/前端时阅读 `docs/DEPLOYMENT_DOCKER.md` 和 `web-ui/README.md`。
2. 先说明任务涉及的模块、接口和数据迁移，再修改代码。禁止未经说明跨层改动。
3. Core 不得依赖具体平台；Collector 通过 Provider 接口访问平台能力。新增平台只新增 adapter、注册组合、测试和构建目标。
4. Agent 与 Server 使用版本化协议。不得随意破坏字段语义；不兼容变化必须升级协议版本并提供迁移策略。
5. 新通知渠道实现 `notify.Channel` 并注册，不得在规则引擎中写渠道类型分支。发送必须设置超时、错误脱敏、有限重试和投递记录。
6. Web UI 使用 Vue 3 + TypeScript + Vite。页面通过 REST API 工作，不得绕过 API 直接读写后端状态。不要引入重量级 UI 框架；组件保持轻量和可访问。
7. 浏览器管理员认证使用服务端会话 Cookie；API 客户端可使用短时 JWT Bearer Token；Agent Token 只用于 Agent ingest。不得把密码、Agent Token、Webhook secret 写入前端 LocalStorage、日志或普通 API 响应；JWT 只允许由认证令牌接口返回。
8. 所有配置必须校验、设置合理默认值，并在 Web UI 暴露可管理配置；秘密字段保存后不回显，只允许替换。
9. 所有网络请求设置超时；所有 goroutine 有退出路径；共享状态必须考虑竞态。
10. 每项功能必须补充单元测试/集成测试和必要文档。交付前执行 `gofmt -w`、`go test -race ./...`、`go vet ./...`、前端 `npm run build` 和 Docker 构建。
11. 不得声称交叉编译通过就代表目标平台功能完整；平台功能必须在真实系统 CI/测试环境验证。
12. 不得为了快速交付把 TODO、stub 或模拟数据描述成已实现功能。

推荐给 AI 的任务模板：

> 请先阅读 Avenlytic 架构与 AI 开发规范。只实现我指定的功能，列出涉及模块和接口，保持现有分层和协议兼容；补齐测试、文档和前端配置界面。禁止修改无关模块。完成后运行所有后端、前端和构建验证，并明确列出未验证项。

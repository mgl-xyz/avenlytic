# Dependency and Toolchain Baseline

Last verified against upstream release pages: **2026-11-02**.

The project intentionally tracks current stable releases rather than floating `latest` tags. Exact versions are used for repeatable builds; Dependabot checks upstream updates weekly.

| Component | Version | Upstream source |
|---|---:|---|
| Go toolchain | 1.27.3 | https://go.dev/dl/ |
| Node.js (LTS) | 26.13.0 | https://nodejs.org/en/download |
| Vue | 3.5.44 | https://github.com/vuejs/core/releases |
| Vite | 8.3.5 | https://github.com/vitejs/vite/releases |
| `@vitejs/plugin-vue` | 6.0.10 | https://github.com/vitejs/vite-plugin-vue/releases |
| TypeScript | 7.0.2 | https://github.com/microsoft/TypeScript/releases |
| Alpine Linux runtime | 3.23 series | https://alpinelinux.org/releases/ |

## GitHub Actions

| Action | Pinned release |
|---|---:|
| `actions/checkout` | v7.0.1 |
| `actions/setup-go` | v7.0.0 |
| `actions/setup-node` | v7.1.0 |
| `actions/upload-artifact` | v7.0.2 |
| `actions/download-artifact` | v8.0.2 |
| `docker/setup-qemu-action` | v4.4.0 |
| `docker/setup-buildx-action` | v4.4.1 |
| `docker/login-action` | v4.7.0 |
| `docker/metadata-action` | v6.2.0 |
| `docker/build-push-action` | v7.4.0 |
| `softprops/action-gh-release` | v3.0.3 |

The versions above are exact upstream release tags checked on the date shown. GitHub-hosted runners support the Node.js 24 action runtime used by the current action releases. `.github/dependabot.yml` checks GitHub Actions, npm, Docker base images, and Go modules weekly.

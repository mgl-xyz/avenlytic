# Avenlytic Web Console

The production console is built with Vue 3, TypeScript and Vite. It is embedded into the Go Server binary at `cmd/server/web`.
The console supports Chinese and English, plus persistent light and dark themes. The Vite development server proxies `/api` and `/healthz` to the Go server on port 9080.
`src/App.vue` owns authentication, API state, and shared layout. Overview, hosts, alerts, rules, channels, settings, and each dialog are separate components under `src/components/`; `src/types.ts` contains shared UI types.
Administrator authentication is server-backed: the server validates credentials, issues an HttpOnly session cookie, and checks that session on protected API requests. The browser does not authenticate locally.

```bash
npm install
npm run dev       # Vite development server; configure a local API proxy if desired
npm run build     # writes production assets to ../cmd/server/web
```

The Dockerfile and GitHub Actions build the Vue application before compiling the Go Server. The checked-in `cmd/server/web` directory is a no-build fallback console for direct `go run` development; it provides the same core pages and cookie-based first-run login, but the Docker/release build replaces it with the Vue bundle.

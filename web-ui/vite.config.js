import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath } from 'node:url'
import { request as httpRequest } from 'node:http'

function uiGateDevPlugin() {
  return {
    name: 'ui-gate-dev',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        if (req.method !== 'GET' || req.url?.startsWith('/api/') || req.url?.startsWith('/healthz')) {
          next()
          return
        }
        const path = new URL(req.url || '/', 'http://localhost').pathname
        const target = new URL('http://127.0.0.1:9080/api/v1/ui-gate/entry')
        target.searchParams.set('path', path)
        const check = httpRequest(target, { headers: { cookie: req.headers.cookie || '' } }, gateResponse => {
          if (gateResponse.statusCode === 204) {
            gateResponse.resume()
            next()
            return
          }
          res.statusCode = gateResponse.statusCode || 404
          for (const header of ['location', 'set-cookie', 'cache-control', 'content-type']) {
            const value = gateResponse.headers[header]
            if (value) res.setHeader(header, value)
          }
          gateResponse.pipe(res)
        })
        check.on('error', () => {
          res.statusCode = 502
          res.end('UI gate check failed')
        })
        check.end()
      })
    },
  }
}

export default defineConfig({
  plugins: [vue(), uiGateDevPlugin()],
  base: '/',
  server: { proxy: { '/api': 'http://127.0.0.1:9080', '/healthz': 'http://127.0.0.1:9080' } },
  build: { outDir: fileURLToPath(new URL('../cmd/server/web', import.meta.url)), emptyOutDir: true, sourcemap: false },
})

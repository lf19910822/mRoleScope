import { fileURLToPath, URL } from 'node:url'

import { defineConfig, loadEnv, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

import { localAccessEntryPath } from './src/lib/local-access-entry'

function localAccessEntryPlugin(): Plugin {
  return {
    name: 'mrolescope-local-access-entry',
    configureServer(server) {
      server.middlewares.use((request, _response, next) => {
        if (request.method === 'GET' || request.method === 'HEAD') {
          const accessEntryPath = localAccessEntryPath(request.url)
          if (accessEntryPath) request.url = accessEntryPath
        }
        next()
      })
    },
  }
}

const env = loadEnv(process.env.NODE_ENV ?? 'development', process.cwd(), 'VITE_')
const apiProxyTarget =
  process.env.VITE_API_PROXY_TARGET ??
  env.VITE_API_PROXY_TARGET ??
  (process.env.NODE_ENV === 'production' ? undefined : 'http://127.0.0.1:20080')
const enableDevtools = process.env.VITE_ENABLE_DEVTOOLS ?? env.VITE_ENABLE_DEVTOOLS

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    localAccessEntryPlugin(),
    vue(),
    ...(enableDevtools === 'true' ? [vueDevTools()] : []),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  // The local UI always uses one loopback-only entry point. Production keeps
  // the Vue application and /api/ same-origin through Web Edge; local
  // development defaults to the current read-only API tunnel so a missing
  // shell environment variable cannot silently fall back to the SPA HTML.
  server: {
    host: '127.0.0.1',
    port: 5175,
    strictPort: true,
    proxy: apiProxyTarget
      ? {
          '/api': {
            target: apiProxyTarget,
            changeOrigin: true,
          },
        }
      : undefined,
  },
})

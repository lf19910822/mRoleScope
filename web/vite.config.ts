import { fileURLToPath, URL } from 'node:url'

import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

import { localAccessEntryPath } from './src/lib/local-access-entry'

const apiProxyTarget = process.env.VITE_API_PROXY_TARGET

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

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    localAccessEntryPlugin(),
    vue(),
    ...(process.env.VITE_ENABLE_DEVTOOLS === 'true' ? [vueDevTools()] : []),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  // The local UI always uses one loopback-only entry point. Production keeps
  // the Vue application and /api/ same-origin through Web Edge; locally the
  // optional proxy must target the current read-only API, never an old fixture
  // service that may silently ignore newly added filters.
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

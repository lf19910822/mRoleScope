import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

const apiProxyTarget = process.env.VITE_API_PROXY_TARGET

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), ...(process.env.VITE_ENABLE_DEVTOOLS === 'true' ? [vueDevTools()] : [])],
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

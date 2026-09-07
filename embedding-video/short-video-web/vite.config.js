import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5174,
    host: true,
    allowedHosts: [
      "na6eaea4.natappfree.cc"
    ],
    proxy: {
      '/api': {
        target: process.env.VITE_PROXY_TARGET || 'http://localhost:8081',
        changeOrigin: true,
      },
      '/videos': {
        target: process.env.VITE_PROXY_TARGET || 'http://localhost:8081',
        changeOrigin: true,
      },
    },
  },
})

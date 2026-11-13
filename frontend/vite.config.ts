import path from 'node:path'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import wails from '@wailsio/runtime/plugins/vite'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue(), tailwindcss(), wails('./src/bindings')],
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, './src'),
    },
  },
  server: {
    // Wails proxies to the dev server over IPv4 only; Vite's default `localhost` can bind just ::1.
    host: '127.0.0.1',
    // `wails3 dev` points the app at this port.
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
})

import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const outDir = fileURLToPath(new URL('../internal/server/app/dist', import.meta.url))

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue()
  ],
  build: {
    outDir,
    emptyOutDir: true,
    chunkSizeWarningLimit: 1500,
    rollupOptions: {
      output: {
        manualChunks: {
          'vue-vendor': ['vue', 'vue-router'],
        }
      }
    }
  }
})

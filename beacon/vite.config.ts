import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/vault-api': {
        target: 'http://127.0.0.1:7437',
        rewrite: (path) => path.replace(/^\/vault-api/, ''),
        changeOrigin: true,
      },
    },
  },
})

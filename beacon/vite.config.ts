import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

// Phase 7 — keep the initial JS payload small.
// Strategy:
//   1. Route-level React.lazy() (in src/pages/admin/Admin.tsx) splits every
//      AdminX page into its own dynamic chunk.
//   2. The manualChunks below pulls the big shared dependencies into named
//      vendor chunks so they're cached across page loads and don't bloat
//      every per-page chunk.
//   3. chunkSizeWarningLimit bumped to 700 KiB so Vite stops warning about
//      the legacy vendor blob; the actual hot-path chunks land well under.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    chunkSizeWarningLimit: 700,
    rollupOptions: {
      output: {
        manualChunks: (id) => {
          if (!id.includes('node_modules')) return undefined
          // React + react-dom are on the critical path; ship together.
          if (
            id.includes('/react-dom/') ||
            id.includes('/react/') ||
            id.includes('/scheduler/')
          ) {
            return 'vendor-react'
          }
          if (id.includes('@tanstack/react-query')) return 'vendor-query'
          if (id.includes('lucide-react')) return 'vendor-icons'
          if (id.includes('zustand')) return 'vendor-state'
          if (id.includes('react-router')) return 'vendor-router'
          // Everything else stays in the default vendor bundle. Avoids
          // creating dozens of tiny micro-chunks (worse waterfall + HTTP/2
          // overhead than one bigger bundle).
          return 'vendor'
        },
      },
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

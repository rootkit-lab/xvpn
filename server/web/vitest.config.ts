import path from 'node:path'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

const rootDir = import.meta.dirname

// Config isolada do Vitest (Fase 15) — evita conflitar o tipagem de
// `test` com o vite.config.ts de build (Vite 8 / rolldown).
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(rootDir, './src'),
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
  },
})

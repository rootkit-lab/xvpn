import path from 'node:path'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

const rootDir = import.meta.dirname

// Config isolada do Vitest (Fase 15) — evita conflitar o tipagem de
// `test` com o vite.config.ts de build (Vite 8 / rolldown).
export default defineConfig({
  plugins: [react()],
  esbuild: {
    jsx: 'automatic',
  },
  resolve: {
    alias: {
      '@': path.resolve(rootDir, './src'),
      '@xvpn/ui': path.resolve(rootDir, '../../shared/ui'),
      react: path.resolve(rootDir, './node_modules/react'),
      'react/jsx-runtime': path.resolve(rootDir, './node_modules/react/jsx-runtime.js'),
      'react/jsx-dev-runtime': path.resolve(rootDir, './node_modules/react/jsx-dev-runtime.js'),
      'react-dom': path.resolve(rootDir, './node_modules/react-dom'),
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
  },
})

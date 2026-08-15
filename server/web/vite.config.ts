import path from 'node:path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const rootDir = import.meta.dirname

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(rootDir, './src'),
    },
  },
  build: {
    // Sai direto dentro do pacote Go que faz o embed (server/internal/webui/dist)
    // em vez de server/web/dist — go:embed não aceita ".." no path, então o
    // diretório embutido precisa estar dentro da árvore do próprio pacote Go
    // (ver server/internal/webui/webui.go e PLAN.md §6.3). Conteúdo gerado
    // fica fora do Git, exceto um placeholder (ver .gitignore e §11.1).
    outDir: path.resolve(rootDir, '../internal/webui/dist'),
    // false de propósito: outDir também guarda o placeholder.txt commitado
    // (ver .gitignore) — "true" limparia a pasta inteira a cada build e
    // apagaria o placeholder do working tree local.
    emptyOutDir: false,
  },
  server: {
    // Em dev, o painel roda separado do Go; proxy evita CORS e replica o
    // comportamento de produção (Nginx -> mesmo host).
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        ws: true,
      },
    },
  },
})

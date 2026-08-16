import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { ThemeProvider } from 'next-themes'
import '@fontsource/outfit/400.css'
import '@fontsource/outfit/500.css'
import '@fontsource/outfit/600.css'
import '@fontsource/outfit/700.css'
import '@xvpn/ui/scss/index.scss'
import './index.css'
import App from './App.tsx'
import { Toaster } from '@/components/ui/sonner'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    {/* Painel é sempre escuro por design (ver index.css) — forcedTheme
        evita qualquer flash de tema claro e mantém o Toaster (sonner)
        consistente com o resto da UI. */}
    <ThemeProvider attribute="class" forcedTheme="dark">
      <App />
      <Toaster richColors />
    </ThemeProvider>
  </StrictMode>,
)

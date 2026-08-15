import { useEffect, useRef, useState, type FormEvent } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { Loader2, X } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const LAST_USER_KEY = 'xvpn.lastUsername'

export function loadLastUsername(): string {
  try {
    return localStorage.getItem(LAST_USER_KEY) ?? ''
  } catch {
    return ''
  }
}

export function saveLastUsername(username: string) {
  try {
    localStorage.setItem(LAST_USER_KEY, username)
  } catch {
    // ignore quota / private mode
  }
}

interface CredentialPromptProps {
  open: boolean
  serverBaseURL: string
  submitting?: boolean
  error?: string | null
  onCancel: () => void
  onSubmit: (username: string, password: string) => void
}

/** Sheet estilo AnyConnect: usuário/senha do painel antes de subir o túnel. */
export function CredentialPrompt({
  open,
  serverBaseURL,
  submitting = false,
  error = null,
  onCancel,
  onSubmit,
}: CredentialPromptProps) {
  const [username, setUsername] = useState(loadLastUsername)
  const [password, setPassword] = useState('')
  const userRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!open) return
    setUsername(loadLastUsername())
    setPassword('')
    const t = window.setTimeout(() => {
      const el = userRef.current
      if (!el) return
      if (el.value) {
        document.getElementById('xvpn-connect-password')?.focus()
      } else {
        el.focus()
      }
    }, 80)
    return () => window.clearTimeout(t)
  }, [open])

  function submit(e: FormEvent) {
    e.preventDefault()
    if (submitting) return
    onSubmit(username.trim(), password)
  }

  const host = serverBaseURL.replace(/^https?:\/\//, '')

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          key="credential-overlay"
          className="absolute inset-0 z-50 flex items-center justify-center p-5"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.22, ease: [0.22, 1, 0.36, 1] }}
        >
          {/* Scrim opaco — evita bleed dos complications por baixo */}
          <motion.button
            type="button"
            aria-label="Fechar autenticação"
            className="absolute inset-0 bg-[oklch(0.11_0.012_260/0.92)] backdrop-blur-md"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            disabled={submitting}
            onClick={() => {
              if (!submitting) onCancel()
            }}
          />

          <motion.div
            role="dialog"
            aria-modal="true"
            aria-labelledby="xvpn-connect-title"
            initial={{ opacity: 0, y: 28, scale: 0.94 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 12, scale: 0.96 }}
            transition={{ type: 'spring', stiffness: 380, damping: 30, mass: 0.85 }}
            className="relative z-10 w-full max-w-sm rounded-[22px] border border-white/10 bg-[oklch(0.17_0.014_260)] p-4 shadow-[0_24px_64px_-16px_oklch(0_0_0/0.75)]"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mb-4 flex items-start justify-between gap-2">
              <div>
                <h2 id="xvpn-connect-title" className="font-display text-[17px] font-semibold tracking-tight">
                  Autenticar
                </h2>
                <p className="mt-1 font-display text-[12px] leading-snug text-muted-foreground">
                  Conta do painel{host ? ` · ${host}` : ''}
                </p>
              </div>
              <button
                type="button"
                onClick={onCancel}
                disabled={submitting}
                aria-label="Cancelar"
                className="flex size-8 shrink-0 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-white/10 hover:text-foreground disabled:opacity-40"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={submit} className="flex flex-col gap-3">
              <div className="flex flex-col gap-1.5">
                <Label
                  htmlFor="xvpn-connect-username"
                  className="font-display text-[10px] uppercase tracking-[0.14em] text-muted-foreground/80"
                >
                  Usuário
                </Label>
                <Input
                  ref={userRef}
                  id="xvpn-connect-username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  autoComplete="username"
                  className="rounded-xl border-white/10 bg-black/35"
                  required
                  disabled={submitting}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label
                  htmlFor="xvpn-connect-password"
                  className="font-display text-[10px] uppercase tracking-[0.14em] text-muted-foreground/80"
                >
                  Senha
                </Label>
                <Input
                  id="xvpn-connect-password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  autoComplete="current-password"
                  className="rounded-xl border-white/10 bg-black/35"
                  required
                  disabled={submitting}
                />
              </div>

              <AnimatePresence mode="wait">
                {error && (
                  <motion.p
                    key={error}
                    initial={{ opacity: 0, y: -4 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0 }}
                    className="font-display text-[12px] text-destructive"
                  >
                    {error}
                  </motion.p>
                )}
              </AnimatePresence>

              <Button
                type="submit"
                disabled={submitting}
                className="mt-1 h-11 rounded-xl font-display text-[15px] font-semibold"
              >
                {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                {submitting ? 'Verificando…' : 'Conectar'}
              </Button>
            </form>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}

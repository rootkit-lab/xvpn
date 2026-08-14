import { useState, type FormEvent } from 'react'
import { Navigate, useLocation, useNavigate, Link } from 'react-router-dom'
import { motion } from 'framer-motion'
import { ShieldCheck } from 'lucide-react'
import { useAuth } from '@/lib/auth-context'
import { ApiError } from '@/lib/api'
import { defaultRouteForRole } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { PageFallback } from '@/components/layout/page-fallback'

export function LoginPage() {
  const { isAuthenticated, isLoadingUser, user, login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  // Já tem token válido de uma sessão anterior: espera o papel carregar
  // (evita mandar todo mundo pro /dashboard e só depois "saltar" pro
  // /portal se for member) antes de decidir o redirect.
  if (isAuthenticated && isLoadingUser) {
    return <PageFallback />
  }
  if (isAuthenticated) {
    const redirectTo = (location.state as { from?: string } | null)?.from ?? defaultRouteForRole(user?.role ?? 'member')
    return <Navigate to={redirectTo} replace />
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      const loggedInUser = await login(username, password)
      navigate(defaultRouteForRole(loggedInUser.role), { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Falha ao conectar ao servidor')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="dot-grid relative flex min-h-svh items-center justify-center overflow-hidden bg-background p-4">
      <div className="glow-blob pointer-events-none absolute top-1/2 left-1/2 h-[480px] w-[480px] -translate-x-1/2 -translate-y-1/2" />
      <motion.div
        initial={{ opacity: 0, y: 12, scale: 0.98 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ duration: 0.4, ease: 'easeOut' }}
        className="relative z-10 w-full max-w-sm"
      >
        <Card className="cyber-frame border-white/5 bg-card/80 backdrop-blur">
          <CardHeader className="items-center text-center">
            {/* Losango "Secured" — assinatura cyber: quadrado girado 45°
                com glow, ícone de escudo no centro. Substitui o logo
                flat pré-redesign. */}
            <div className="relative mb-2 flex size-16 items-center justify-center">
              <div className="cyber-diamond absolute inset-0 m-auto size-12 bg-primary/10" />
              <ShieldCheck className="relative size-7 text-primary drop-shadow-[0_0_10px_var(--color-glow)]" />
            </div>
            <CardTitle className="text-xl tracking-tight">XVPN — Painel</CardTitle>
            <CardDescription className="hud-label text-muted-foreground/80">
              acesso seguro · controle administrativo
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
              <div className="flex flex-col gap-2">
                <Label htmlFor="username" className="hud-label text-muted-foreground/80">
                  Usuário
                </Label>
                <Input
                  id="username"
                  autoComplete="username"
                  required
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="font-mono"
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="password" className="hud-label text-muted-foreground/80">
                  Senha
                </Label>
                <Input
                  id="password"
                  type="password"
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="font-mono"
                />
              </div>
              {error && (
                <p className="hud-label flex items-center gap-2 text-destructive">
                  <span className="size-1.5 rounded-full bg-destructive" />
                  {error}
                </p>
              )}
              <Button type="submit" disabled={submitting} className="mt-2 rounded-md font-mono tracking-wide">
                {submitting ? 'Autenticando…' : 'Entrar →'}
              </Button>
            </form>
          </CardContent>
        </Card>
        <p className="mt-4 text-center text-sm text-muted-foreground">
          <Link to="/" className="underline underline-offset-4 hover:text-foreground">
            ← Voltar para a página inicial
          </Link>
        </p>
      </motion.div>
    </div>
  )
}

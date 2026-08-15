import { useState, type FormEvent } from 'react'
import { Navigate, useLocation, useNavigate, Link } from 'react-router-dom'
import { motion } from 'framer-motion'
import { ShieldCheck, UserRound } from 'lucide-react'
import { useAuth } from '@/lib/auth-context'
import { ApiError } from '@/lib/api'
import { defaultRouteForRole } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { PageFallback } from '@/components/layout/page-fallback'

export function LoginPage({ variant = 'user' }: { variant?: 'user' | 'admin' }) {
  const { isAuthenticated, isLoadingUser, user, login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const isAdminLogin = variant === 'admin'

  if (isAuthenticated && isLoadingUser) {
    return <PageFallback />
  }
  if (isAuthenticated) {
    const from = (location.state as { from?: string } | null)?.from
    const redirectTo = from ?? defaultRouteForRole(user?.role ?? 'member')
    return <Navigate to={redirectTo} replace />
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      const loggedInUser = await login(username, password)
      const from = (location.state as { from?: string } | null)?.from
      // Member que entrou pelo login de admin não fica no /admin — vai pro /app.
      // Viewer+ que entrou pelo login de usuário pode ir ao destino default (admin)
      // ou ao `from` se for rota permitida.
      let dest = from ?? defaultRouteForRole(loggedInUser.role)
      if (loggedInUser.role === 'member' && dest.startsWith('/admin')) {
        dest = '/app'
      }
      if (loggedInUser.role !== 'member' && isAdminLogin && !from) {
        dest = '/admin'
      }
      navigate(dest, { replace: true })
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
            <div className="relative mb-2 flex size-16 items-center justify-center">
              <div className="cyber-diamond absolute inset-0 m-auto size-12 bg-primary/10" />
              {isAdminLogin ? (
                <ShieldCheck className="relative size-7 text-primary drop-shadow-[0_0_10px_var(--color-glow)]" />
              ) : (
                <UserRound className="relative size-7 text-primary drop-shadow-[0_0_10px_var(--color-glow)]" />
              )}
            </div>
            <CardTitle className="text-xl tracking-tight">
              {isAdminLogin ? 'XVPN — Administração' : 'XVPN — Meu espaço'}
            </CardTitle>
            <CardDescription className="hud-label text-muted-foreground/80">
              {isAdminLogin
                ? 'acesso seguro · painel do sistema'
                : 'dispositivos · downloads · apps'}
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
          {isAdminLogin ? (
            <>
              Conta de membro?{' '}
              <Link to="/app/login" className="underline underline-offset-4 hover:text-foreground">
                Entrar no meu espaço
              </Link>
            </>
          ) : (
            <>
              Operador do sistema?{' '}
              <Link to="/admin/login" className="underline underline-offset-4 hover:text-foreground">
                Administração
              </Link>
            </>
          )}
        </p>
        <p className="mt-2 text-center text-sm text-muted-foreground">
          <Link to="/" className="underline underline-offset-4 hover:text-foreground">
            ← Voltar para a página inicial
          </Link>
        </p>
      </motion.div>
    </div>
  )
}

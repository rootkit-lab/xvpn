import { useState, type FormEvent } from 'react'
import { Navigate, useLocation, useNavigate, Link } from 'react-router-dom'
import { motion } from 'framer-motion'
import { ShieldCheck, UserRound } from 'lucide-react'
import { useAuth } from '@/lib/auth-context'
import { PANEL_ORIGIN } from '@/lib/product-host'
import { ApiError } from '@/lib/api'
import { defaultRouteForRole } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { PageFallback } from '@/components/layout/page-fallback'

export function LoginPage({ variant = 'user' }: { variant?: 'user' | 'admin' | 'store' }) {
  const { isAuthenticated, isLoadingUser, user, login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const isAdminLogin = variant === 'admin'
  const isStoreLogin = variant === 'store'

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
      // Member que entrou pelo login de admin não fica no /admin — vai pro /my.
      let dest = from ?? defaultRouteForRole(loggedInUser.role)
      if (isStoreLogin) {
        dest = from ?? '/'
      } else if (loggedInUser.role === 'member' && dest.startsWith('/admin')) {
        dest = '/my'
      } else if (loggedInUser.role !== 'member' && isAdminLogin && !from) {
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
    <div className="watch-face relative flex min-h-svh items-center justify-center overflow-hidden p-4">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      <div className="glow-blob pointer-events-none absolute top-1/2 left-1/2 h-[480px] w-[480px] -translate-x-1/2 -translate-y-1/2" />
      <motion.div
        initial={{ opacity: 0, y: 12, scale: 0.98 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ duration: 0.4, ease: 'easeOut' }}
        className="relative z-10 w-full max-w-sm"
      >
        <Card>
          <CardHeader className="items-center text-center">
            <div className="relative mb-2 flex size-16 items-center justify-center">
              <div className="cyber-diamond absolute inset-0 m-auto size-12 bg-primary/10" />
              {isAdminLogin ? (
                <ShieldCheck className="relative size-7 text-primary drop-shadow-[0_0_10px_var(--color-glow)]" />
              ) : (
                <UserRound className="relative size-7 text-primary drop-shadow-[0_0_10px_var(--color-glow)]" />
              )}
            </div>
            <CardTitle className="font-display text-xl tracking-tight">
              {isAdminLogin ? 'xvpn — Administração' : isStoreLogin ? 'ihuull' : 'xvpn'}
            </CardTitle>
            <CardDescription className="hud-label text-muted-foreground/75">
              {isAdminLogin
                ? 'acesso seguro · painel do sistema'
                : isStoreLogin
                  ? 'marketplace · xdriver'
                  : 'dispositivos · marketplace'}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
              <div className="flex flex-col gap-2">
                <Label htmlFor="username" className="hud-label text-muted-foreground/75">
                  Usuário
                </Label>
                <Input
                  id="username"
                  autoComplete="username"
                  required
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="password" className="hud-label text-muted-foreground/75">
                  Senha
                </Label>
                <Input
                  id="password"
                  type="password"
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </div>
              {error && (
                <p className="hud-label flex items-center gap-2 text-destructive">
                  <span className="size-1.5 rounded-full bg-destructive" />
                  {error}
                </p>
              )}
              <Button type="submit" disabled={submitting} size="lg" className="mt-2 w-full">
                {submitting ? 'Autenticando…' : 'Entrar →'}
              </Button>
            </form>
          </CardContent>
        </Card>
        {!isStoreLogin && (
          <p className="mt-4 text-center text-sm text-muted-foreground">
            {isAdminLogin ? (
              <>
                Conta de membro?{' '}
                <Link to="/my/login" className="underline underline-offset-4 hover:text-foreground">
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
        )}
        <p className="mt-2 text-center text-sm text-muted-foreground">
          {isStoreLogin ? (
            <a href={PANEL_ORIGIN} className="underline underline-offset-4 hover:text-foreground">
              ← Painel XVPN
            </a>
          ) : (
            <Link to="/" className="underline underline-offset-4 hover:text-foreground">
              ← Voltar para a página inicial
            </Link>
          )}
        </p>
      </motion.div>
    </div>
  )
}

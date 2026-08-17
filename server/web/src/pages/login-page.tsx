import { useState, type FormEvent } from 'react'
import { Navigate, useLocation, useNavigate, Link } from 'react-router-dom'
import { Eye, EyeOff } from 'lucide-react'
import { ProductMark } from '@xvpn/ui/react/product-mark'
import { PRODUCT_META } from '@xvpn/ui/react/products'
import { Complication } from '@xvpn/ui/react/complication'
import { IconButton } from '@xvpn/ui/react/icon-button'
import { useAuth } from '@/lib/auth-context'
import { ApiError, getToken } from '@/lib/api'
import { isLoggedOutParam, productKind, safeReturnURL, ssoHandoff, ssoHandoffContinueURL } from '@/lib/product-host'
import { defaultRouteForRole } from '@/lib/roles'
import { loginCopy, loginHomeLink, type LoginVariant } from '@/lib/login-copy'
import { NetworkGlobe } from '@/components/network-globe'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { PageFallback } from '@/components/layout/page-fallback'

/** JWE do login (memória) ou form HTML no xauth — nunca JSON do cookie. */
function continueSSO(role: string, returnTo: string | null) {
  const token = getToken()
  if (token) {
    ssoHandoff(role, returnTo, token)
    return
  }
  window.location.replace(ssoHandoffContinueURL(role, returnTo))
}

export function LoginPage({ variant = 'user' }: { variant?: LoginVariant }) {
  const { isAuthenticated, isLoadingUser, user, login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const isAdminLogin = variant === 'admin'
  const isStoreLogin = variant === 'store'
  const isSSO = variant === 'sso' || productKind() === 'xauth'
  const copyVariant: LoginVariant = isSSO && variant !== 'admin' && variant !== 'store' ? 'sso' : variant
  const returnTo = safeReturnURL(new URLSearchParams(location.search).get('return'))
  const loggedOut = isLoggedOutParam(location.search)
  const showContinue = isSSO && isAuthenticated && !isLoadingUser && !loggedOut
  const copy = loginCopy(copyVariant)
  const home = loginHomeLink(copyVariant)
  const meta = PRODUCT_META[copy.product]

  if (isAuthenticated && isLoadingUser && !isSSO) {
    return <PageFallback />
  }
  if (isAuthenticated && !isSSO) {
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
      if (isSSO) {
        continueSSO(loggedInUser.role, returnTo)
        return
      }
      const from = (location.state as { from?: string } | null)?.from
      // Member que entrou pelo login de admin não fica no /admin — vai pro portal.
      let dest = from ?? defaultRouteForRole(loggedInUser.role)
      if (isStoreLogin) {
        dest = from ?? '/'
      } else if (loggedInUser.role === 'member' && dest.startsWith('/admin')) {
        dest = defaultRouteForRole('member')
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
    <div data-product={copy.product} className="watch-face relative min-h-svh overflow-hidden">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />

      <div className="relative z-10 grid min-h-svh lg:grid-cols-[minmax(0,2fr)_minmax(24rem,3fr)]">
        <aside className="relative hidden overflow-hidden border-r border-white/8 lg:flex">
          <div className="dot-grid pointer-events-none absolute inset-0 opacity-40" aria-hidden="true" />
          <div
            className="glow-blob pointer-events-none absolute -bottom-28 -left-20 h-[440px] w-[440px]"
            aria-hidden="true"
          />

          <div className="relative z-10 flex w-full flex-col px-10 py-10">
            <a href={home.href} className="flex items-center gap-2.5">
              <span className="icon-well flex size-9 items-center justify-center rounded-[10px]">
                <ProductMark product={copy.product} className="size-4" />
              </span>
              <span className="leading-tight">
                <span className="font-display block text-[15px] font-semibold tracking-tight">{meta.label}</span>
                <span className="hud-label text-muted-foreground/70">{copy.kicker}</span>
              </span>
            </a>

            <div className="relative my-auto">
              <NetworkGlobe className="mx-auto h-56 w-full max-w-md opacity-50" />
              <Complication className="absolute inset-x-6 bottom-1 border border-white/10 px-4 py-3.5">
                <p className="hud-label text-muted-foreground/70">Sessão</p>
                <p className="mt-1 font-display text-sm font-semibold">Cookie .ihuull.com</p>
                <p className="mt-1 text-sm leading-relaxed text-muted-foreground">
                  Um login vale nos hosts da organização. A chave WireGuard continua só no seu aparelho.
                </p>
              </Complication>
            </div>

            <div className="max-w-md pt-8">
              <h2 className="font-display text-3xl font-semibold tracking-tight text-balance text-glow">
                {copy.brandTitle}
              </h2>
              <p className="mt-2 text-sm leading-relaxed text-muted-foreground">{copy.brandBody}</p>
            </div>
          </div>
        </aside>

        <main className="flex flex-col justify-center px-6 py-12 sm:px-12 lg:px-16">
          <div className="mx-auto w-full max-w-sm">
            <div className="mb-8 flex items-center gap-2.5 lg:hidden">
              <span className="icon-well flex size-9 items-center justify-center rounded-[10px]">
                <ProductMark product={copy.product} className="size-4" />
              </span>
              <span className="font-display text-[15px] font-semibold">{meta.label}</span>
            </div>

            <h1 className="font-display text-2xl font-semibold tracking-tight sm:text-[1.75rem]">{copy.title}</h1>
            <p className="mt-1.5 text-sm text-muted-foreground">{copy.subtitle}</p>

            {showContinue && (
              <div className="mt-8 flex flex-col gap-3">
                <Button
                  type="button"
                  size="lg"
                  className="w-full"
                  onClick={() => continueSSO(user?.role ?? 'member', returnTo)}
                >
                  Continuar como {user?.username ?? 'usuário'}
                </Button>
                <div className="flex items-center gap-3 text-[11px] text-muted-foreground">
                  <span className="h-px flex-1 bg-white/10" />
                  ou entre com outra conta
                  <span className="h-px flex-1 bg-white/10" />
                </div>
              </div>
            )}

            <form
              className={showContinue ? 'mt-4 flex flex-col gap-4' : 'mt-8 flex flex-col gap-4'}
              onSubmit={handleSubmit}
            >
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="username" className="hud-label text-muted-foreground/75">
                  Usuário
                </Label>
                <Input
                  id="username"
                  name="username"
                  autoComplete="username"
                  autoFocus
                  spellCheck={false}
                  required
                  placeholder="sua conta"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="password" className="hud-label text-muted-foreground/75">
                  Senha
                </Label>
                <div className="relative">
                  <Input
                    id="password"
                    name="password"
                    type={showPassword ? 'text' : 'password'}
                    autoComplete="current-password"
                    required
                    className="pr-11"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                  />
                  <IconButton
                    label={showPassword ? 'Ocultar senha' : 'Mostrar senha'}
                    onClick={() => setShowPassword((v) => !v)}
                    className="absolute inset-y-0 right-1 my-auto"
                  >
                    {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
                  </IconButton>
                </div>
              </div>
              {error && (
                <p className="text-sm text-destructive" role="alert">
                  {error}
                </p>
              )}
              <Button type="submit" disabled={submitting} size="lg" className="mt-1 w-full">
                {submitting ? 'Autenticando…' : 'Entrar'}
              </Button>
            </form>

            {!isStoreLogin && !isSSO && (
              <p className="mt-6 text-center text-sm text-muted-foreground">
                {isAdminLogin ? (
                  <>
                    Conta de membro?{' '}
                    <Link to="/my/login" className="text-foreground underline underline-offset-4">
                      Entrar no meu espaço
                    </Link>
                  </>
                ) : (
                  <>
                    Operador do sistema?{' '}
                    <Link to="/admin/login" className="text-foreground underline underline-offset-4">
                      Administração
                    </Link>
                  </>
                )}
              </p>
            )}
            <p className="mt-3 text-center text-sm text-muted-foreground">
              {home.external ? (
                <a href={home.href} className="underline underline-offset-4 hover:text-foreground">
                  {home.label}
                </a>
              ) : (
                <Link to={home.href} className="underline underline-offset-4 hover:text-foreground">
                  {home.label}
                </Link>
              )}
            </p>
            <p className="mt-8 text-center text-[11px] leading-relaxed text-muted-foreground/70">
              Acesso por convite da organização. Sem cadastro público.
            </p>
          </div>
        </main>
      </div>
    </div>
  )
}

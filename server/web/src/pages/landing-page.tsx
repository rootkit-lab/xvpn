import { useState, type FormEvent } from 'react'
import { Link, Navigate } from 'react-router-dom'
import { motion } from 'framer-motion'
import { Laptop, Lock, Network, ShieldCheck, Wifi, Zap } from 'lucide-react'
import { ProductHeader } from '@xvpn/ui/react/product-header'
import { useAuth } from '@/lib/auth-context'
import { api, ApiError } from '@/lib/api'
import { headerProduct } from '@/lib/product-host'
import { defaultRouteForRole } from '@/lib/roles'
import { NetworkGlobe } from '@/components/network-globe'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const FEATURES = [
  {
    icon: Network,
    title: 'Sua própria rede privada',
    description: 'Todos os seus dispositivos na mesma rede, como se estivessem no mesmo escritório — em qualquer lugar do mundo.',
  },
  {
    icon: Zap,
    title: 'Rápida e estável',
    description: 'Construída sobre WireGuard: handshakes quase instantâneos, baixa latência e reconexão automática se a conexão cair.',
  },
  {
    icon: Lock,
    title: 'Segurança de ponta a ponta',
    description: 'Sua chave privada nunca sai do seu dispositivo. Kill switch e split-tunnel opcionais para controlar exatamente o que passa pela VPN.',
  },
  {
    icon: Laptop,
    title: 'Cliente para Windows e Linux',
    description: 'Aplicativo desktop nativo com ícone de bandeja, diagnóstico embutido e conexão em um clique.',
  },
  {
    icon: Wifi,
    title: 'Saída própria (exit node)',
    description: 'Navegue com o IP do seu próprio servidor, sem depender de provedores terceiros de VPN.',
  },
  {
    icon: ShieldCheck,
    title: 'Compartilhamento de arquivos',
    description: 'Acesso a uma unidade de rede e a um navegador de arquivos, disponíveis só para quem está conectado à VPN.',
  },
]

const fadeUp = {
  hidden: { opacity: 0, y: 16 },
  show: { opacity: 1, y: 0 },
}

export function LandingPage() {
  const { isAuthenticated, isLoadingUser, user } = useAuth()
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [message, setMessage] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [submitted, setSubmitted] = useState(false)

  const product = headerProduct()

  if (isAuthenticated && !isLoadingUser) {
    return <Navigate to={defaultRouteForRole(user?.role ?? 'member')} replace />
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await api.joinWaitlist(name, email, message)
      setSubmitted(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Falha ao enviar seu cadastro. Tente novamente em alguns minutos.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div data-product={product} className="watch-face relative min-h-svh overflow-hidden">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      <div className="glow-blob pointer-events-none absolute -top-40 left-1/2 h-[560px] w-[560px] -translate-x-1/2" />
      <NetworkGlobe className="pointer-events-none absolute inset-x-0 top-0 mx-auto h-[520px] w-full max-w-3xl opacity-70" />

      <ProductHeader
        product={product}
        href="/"
        trailing={
          <Button variant="ghost" className="rounded-full" asChild>
            <Link to="/my/login">Entrar</Link>
          </Button>
        }
      />

      <main className="relative z-10 mx-auto flex max-w-5xl flex-col gap-16 px-6 pb-20 sm:px-10">
        <motion.section
          className="flex flex-col items-center gap-6 pt-10 text-center"
          initial="hidden"
          animate="show"
          variants={fadeUp}
          transition={{ duration: 0.6, ease: 'easeOut' }}
        >
          <motion.img
            src="/logo-192.png"
            alt=""
            className="size-24 drop-shadow-[0_0_30px_var(--color-glow)]"
            initial={{ scale: 0.85, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            transition={{ duration: 0.7, ease: 'easeOut' }}
          />
          <h1 className="font-display max-w-2xl text-4xl font-bold tracking-tight text-glow sm:text-5xl">
            Sua própria VPN privada, do jeito que devia ser
          </h1>
          <p className="font-display max-w-xl text-lg text-muted-foreground">
            Rede privada pessoal com saída própria, rápida e sob seu controle — sem depender de provedores
            terceiros. Acesso liberado por convite, através da lista de espera abaixo.
          </p>
          <div className="flex flex-wrap items-center justify-center gap-3">
            <Button size="lg" asChild>
              <a href="#waitlist">Entrar na lista de espera</a>
            </Button>
            <Button size="lg" variant="outline" asChild>
              <Link to="/my/login">Já tenho acesso</Link>
            </Button>
          </div>
        </motion.section>

        <motion.section
          className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3"
          initial="hidden"
          whileInView="show"
          viewport={{ once: true, margin: '-40px' }}
          variants={{ show: { transition: { staggerChildren: 0.08 } } }}
        >
          {FEATURES.map(({ icon: Icon, title, description }) => (
            <motion.div key={title} variants={fadeUp} transition={{ duration: 0.45, ease: 'easeOut' }}>
              <Card className="watch-complication-lift h-full">
                <CardHeader>
                  <div className="icon-well mb-2 flex size-11 items-center justify-center rounded-[10px] text-foreground">
                    <Icon className="size-5" />
                  </div>
                  <CardTitle className="font-display text-base">{title}</CardTitle>
                </CardHeader>
                <CardContent>
                  <CardDescription>{description}</CardDescription>
                </CardContent>
              </Card>
            </motion.div>
          ))}
        </motion.section>

        <motion.section
          id="waitlist"
          className="flex justify-center"
          initial="hidden"
          whileInView="show"
          viewport={{ once: true, margin: '-40px' }}
          variants={fadeUp}
          transition={{ duration: 0.5 }}
        >
          <Card className="glow-ring w-full max-w-md">
            <CardHeader>
              <CardTitle className="font-display">Entre na lista de espera</CardTitle>
              <CardDescription>
                O acesso ao XVPN é liberado por aprovação manual. Deixe seus dados e avisamos quando seu
                acesso estiver pronto.
              </CardDescription>
            </CardHeader>
            <CardContent>
              {submitted ? (
                <motion.p
                  initial={{ opacity: 0, scale: 0.96 }}
                  animate={{ opacity: 1, scale: 1 }}
                  className="watch-complication rounded-[12px] p-4 text-sm"
                >
                  Cadastro recebido! Entraremos em contato pelo e-mail informado quando seu acesso for
                  aprovado.
                </motion.p>
              ) : (
                <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="name" className="hud-label text-muted-foreground/75">
                      Nome
                    </Label>
                    <Input id="name" required value={name} onChange={(e) => setName(e.target.value)} />
                  </div>
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="email" className="hud-label text-muted-foreground/75">
                      E-mail
                    </Label>
                    <Input
                      id="email"
                      type="email"
                      required
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                    />
                  </div>
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="message" className="hud-label text-muted-foreground/75">
                      Por que você quer acesso? (opcional)
                    </Label>
                    <Textarea
                      id="message"
                      rows={3}
                      value={message}
                      onChange={(e) => setMessage(e.target.value)}
                    />
                  </div>
                  {error && <p className="text-sm text-destructive">{error}</p>}
                  <Button type="submit" disabled={submitting} size="lg">
                    {submitting ? 'Enviando…' : 'Entrar na lista de espera'}
                  </Button>
                </form>
              )}
            </CardContent>
          </Card>
        </motion.section>
      </main>

      <footer className="chrome-bar relative z-10 border-t border-white/8 px-6 py-6 text-center font-display text-sm text-muted-foreground">
        XVPN — rede privada pessoal.{' '}
        <Link to="/my/login" className="underline underline-offset-4 hover:text-foreground">
          Já tem acesso? Entre aqui.
        </Link>
        {' · '}
        <Link to="/admin/login" className="underline underline-offset-4 hover:text-foreground">
          Administração
        </Link>
      </footer>
    </div>
  )
}

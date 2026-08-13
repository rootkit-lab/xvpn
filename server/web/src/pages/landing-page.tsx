import { useState, type FormEvent } from 'react'
import { Link, Navigate } from 'react-router-dom'
import { Laptop, Lock, Network, ShieldCheck, Wifi, Zap } from 'lucide-react'
import { useAuth } from '@/lib/auth-context'
import { api, ApiError } from '@/lib/api'
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

export function LandingPage() {
  const { isAuthenticated } = useAuth()
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [message, setMessage] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [submitted, setSubmitted] = useState(false)

  if (isAuthenticated) {
    return <Navigate to="/dashboard" replace />
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
    <div className="min-h-svh bg-background">
      <header className="flex items-center justify-between px-6 py-5 sm:px-10">
        <div className="flex items-center gap-2">
          <img src="/logo-192.png" alt="XVPN" className="size-9" />
          <span className="text-lg font-semibold">XVPN</span>
        </div>
        <Button variant="ghost" asChild>
          <Link to="/login">Entrar</Link>
        </Button>
      </header>

      <main className="mx-auto flex max-w-5xl flex-col gap-16 px-6 pb-20 sm:px-10">
        <section className="flex flex-col items-center gap-6 pt-10 text-center">
          <img src="/logo-192.png" alt="" className="size-24" />
          <h1 className="max-w-2xl text-4xl font-bold tracking-tight sm:text-5xl">
            Sua própria VPN privada, do jeito que devia ser
          </h1>
          <p className="max-w-xl text-lg text-muted-foreground">
            Rede privada pessoal com saída própria, rápida e sob seu controle — sem depender de provedores
            terceiros. Acesso liberado por convite, através da lista de espera abaixo.
          </p>
        </section>

        <section className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {FEATURES.map(({ icon: Icon, title, description }) => (
            <Card key={title}>
              <CardHeader>
                <Icon className="mb-2 size-6 text-primary" />
                <CardTitle className="text-base">{title}</CardTitle>
              </CardHeader>
              <CardContent>
                <CardDescription>{description}</CardDescription>
              </CardContent>
            </Card>
          ))}
        </section>

        <section id="waitlist" className="flex justify-center">
          <Card className="w-full max-w-md">
            <CardHeader>
              <CardTitle>Entre na lista de espera</CardTitle>
              <CardDescription>
                O acesso ao XVPN é liberado por aprovação manual. Deixe seus dados e avisamos quando seu
                acesso estiver pronto.
              </CardDescription>
            </CardHeader>
            <CardContent>
              {submitted ? (
                <p className="rounded-md border border-primary/30 bg-primary/10 p-4 text-sm">
                  Cadastro recebido! Entraremos em contato pelo e-mail informado quando seu acesso for
                  aprovado.
                </p>
              ) : (
                <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="name">Nome</Label>
                    <Input id="name" required value={name} onChange={(e) => setName(e.target.value)} />
                  </div>
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="email">E-mail</Label>
                    <Input
                      id="email"
                      type="email"
                      required
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                    />
                  </div>
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="message">Por que você quer acesso? (opcional)</Label>
                    <Textarea
                      id="message"
                      rows={3}
                      value={message}
                      onChange={(e) => setMessage(e.target.value)}
                    />
                  </div>
                  {error && <p className="text-sm text-destructive">{error}</p>}
                  <Button type="submit" disabled={submitting}>
                    {submitting ? 'Enviando…' : 'Entrar na lista de espera'}
                  </Button>
                </form>
              )}
            </CardContent>
          </Card>
        </section>
      </main>

      <footer className="border-t px-6 py-6 text-center text-sm text-muted-foreground">
        XVPN — rede privada pessoal.{' '}
        <Link to="/login" className="underline">
          Já tem acesso? Entre aqui.
        </Link>
      </footer>
    </div>
  )
}

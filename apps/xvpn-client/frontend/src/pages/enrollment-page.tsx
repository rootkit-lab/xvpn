import { useState, type FormEvent } from 'react'
import { motion } from 'framer-motion'
import { Loader2 } from 'lucide-react'

import { VPNService } from '../../bindings/github.com/rootkit-lab/xvpn/client'
import { ConnectionRings } from '@/components/connection-rings'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

interface EnrollmentPageProps {
  onEnrolled: () => void
}

export function EnrollmentPage({ onEnrolled }: EnrollmentPageProps) {
  const [serverBaseURL, setServerBaseURL] = useState('https://vpn.officeempresa.com')
  const [inviteToken, setInviteToken] = useState('')
  const [deviceName, setDeviceName] = useState('')
  const [mtu, setMtu] = useState('')
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setLoading(true)
    setError(null)
    try {
      await VPNService.Enroll({
        serverBaseURL,
        inviteToken,
        deviceName,
        mtu: mtu ? Number(mtu) : 0,
      })
      onEnrolled()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="watch-face relative flex h-full items-center justify-center overflow-hidden p-6">
      <div className="watch-vignette pointer-events-none absolute inset-0" aria-hidden="true" />
      <ConnectionRings className="pointer-events-none absolute left-1/2 top-[-10%] h-56 w-56 -translate-x-1/2 opacity-40" />

      <motion.div
        initial={{ opacity: 0, y: 10, scale: 0.98 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ duration: 0.35, ease: 'easeOut' }}
        className="relative z-10 w-full max-w-sm"
      >
        <Card className="rounded-[22px] border-white/8 bg-card/75 backdrop-blur-xl">
          <CardHeader className="items-center text-center">
            <img src="/logo-192.png" alt="XVPN" className="mb-1 h-12 w-12 rounded-full" />
            <CardTitle className="font-display">Conectar este dispositivo</CardTitle>
            <CardDescription className="font-display">Insira o código de convite gerado no painel web XVPN.</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="flex flex-col gap-4">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="server">Servidor</Label>
                <Input
                  id="server"
                  value={serverBaseURL}
                  onChange={(e) => setServerBaseURL(e.target.value)}
                  placeholder="https://vpn.officeempresa.com"
                  required
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="token">Código de convite</Label>
                <Input
                  id="token"
                  value={inviteToken}
                  onChange={(e) => setInviteToken(e.target.value)}
                  placeholder="Gerado na tela Usuários do painel"
                  autoComplete="off"
                  required
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="deviceName">Nome deste dispositivo</Label>
                <Input
                  id="deviceName"
                  value={deviceName}
                  onChange={(e) => setDeviceName(e.target.value)}
                  placeholder="Ex.: notebook-pessoal"
                  required
                />
              </div>
              <button
                type="button"
                onClick={() => setShowAdvanced((v) => !v)}
                className="text-left text-xs text-muted-foreground underline-offset-2 hover:underline"
              >
                {showAdvanced ? 'Ocultar opções avançadas' : 'Opções avançadas'}
              </button>
              {showAdvanced && (
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="mtu">MTU (opcional)</Label>
                  <Input
                    id="mtu"
                    type="number"
                    value={mtu}
                    onChange={(e) => setMtu(e.target.value)}
                    placeholder="1420 (padrão)"
                  />
                  <p className="text-xs text-muted-foreground">
                    Reduza (ex.: 1200) se já estiver atrás de outra VPN/rede restritiva e a conexão parecer
                    "conectada" mas sem tráfego.
                  </p>
                </div>
              )}
              {error && <p className="text-sm text-destructive">{error}</p>}
              <Button type="submit" disabled={loading} className="mt-1 rounded-full">
                {loading && <Loader2 className="animate-spin" />}
                {loading ? 'Registrando…' : 'Registrar dispositivo'}
              </Button>
            </form>
          </CardContent>
        </Card>
      </motion.div>
    </div>
  )
}

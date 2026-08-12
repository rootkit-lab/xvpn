import { HardDrive } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

export function SharesPage() {
  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold">Compartilhamentos</h1>
        <p className="text-muted-foreground">Diretórios do VPS compartilhados na rede privada.</p>
      </div>

      <Card>
        <CardHeader className="items-center text-center">
          <HardDrive className="mb-2 size-10 text-muted-foreground" />
          <CardTitle className="text-base">Em breve — Fase 5</CardTitle>
          <CardDescription className="max-w-md">
            O gerenciamento de compartilhamentos (Samba + FileBrowser, escopados exclusivamente à interface{' '}
            <code>wg0</code>) chega na Fase 5 do roadmap. Nenhum serviço de arquivos está ativo no servidor ainda.
          </CardDescription>
        </CardHeader>
        <CardContent />
      </Card>
    </div>
  )
}

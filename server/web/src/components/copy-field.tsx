import { Copy } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'

// CopyField exibe um valor sensível (senha gerada, token de convite) com um
// botão de copiar — usado nos poucos lugares em que a API devolve um
// segredo de curta duração, uma única vez (ver frontend-react.mdc: nunca
// deixar recuperável depois).
export function CopyField({ label, value }: { label: string; value: string }) {
  function copy() {
    navigator.clipboard.writeText(value)
    toast.success(`${label} copiado`)
  }

  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs text-muted-foreground">{label}</span>
      <div className="flex items-center gap-2">
        <code className="flex-1 truncate rounded bg-muted px-3 py-1.5 text-sm font-medium">{value}</code>
        <Button type="button" variant="outline" size="icon" onClick={copy}>
          <Copy className="size-4" />
        </Button>
      </div>
    </div>
  )
}

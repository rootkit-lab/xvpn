import { Link } from 'react-router-dom'
import { Users } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

export function XGroupAdminPage() {
  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader className="flex-row items-start gap-3 space-y-0">
          <Users className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
          <div>
            <CardTitle className="text-base">XGroup</CardTitle>
            <CardDescription>
              Rede social da intranet. Membros publicam e seguem em{' '}
              <Link to="/social" className="underline underline-offset-4">
                /social
              </Link>
              . Não há escrita administrativa nesta fatia — o escopo{' '}
              <code className="font-mono text-xs">xgroup</code> reserva a seção para operação futura
              (moderação) sem misturar com peers ou ACL da loja.
            </CardDescription>
          </div>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">
          Fonte única: esta tela vive em <code className="font-mono text-xs">xadmin.corp.ihuull.com/admin/xgroup</code>.
          Não existe host <code className="font-mono text-xs">admin.xgroup</code>.
        </CardContent>
      </Card>
    </div>
  )
}

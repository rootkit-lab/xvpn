import { useCallback, useMemo } from 'react'
import { Shield } from 'lucide-react'
import { api } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import {
  ALL_ROLES,
  canManageRole,
  ROLE_BADGE_VARIANT,
  ROLE_CAPABILITIES,
  ROLE_DESCRIPTIONS,
  ROLE_LABELS,
  ROLE_RANK,
  type Role,
} from '@/lib/roles'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Skeleton } from '@/components/ui/skeleton'

export function RbacPage() {
  const { user: caller } = useAuth()
  const fetchUsers = useCallback(() => api.listUsers({ per_page: 100 }), [])
  const { data: page, loading, error } = usePollingData(fetchUsers, 30_000)
  const users = page?.items

  const counts = useMemo(() => {
    const next: Record<Role, number> = { super_admin: 0, admin: 0, viewer: 0, member: 0 }
    for (const u of users ?? []) next[u.role] += 1
    return next
  }, [users])

  const ranked = [...ALL_ROLES].sort((a, b) => ROLE_RANK[b] - ROLE_RANK[a])

  return (
    <div className="flex flex-col gap-6">
      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {ranked.map((role) => (
          <Card key={role} className="border-white/5 bg-card/60">
            <CardHeader className="pb-2">
              <div className="flex items-center justify-between gap-2">
                <Badge variant={ROLE_BADGE_VARIANT[role]}>{ROLE_LABELS[role]}</Badge>
                <span className="font-mono text-xs text-muted-foreground">rank {ROLE_RANK[role]}</span>
              </div>
              <CardTitle className="pt-2 text-3xl font-semibold tabular-nums">
                {loading || !users ? <Skeleton className="h-8 w-10" /> : counts[role]}
              </CardTitle>
              <CardDescription>{ROLE_DESCRIPTIONS[role]}</CardDescription>
            </CardHeader>
          </Card>
        ))}
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Shield className="size-4" />
            Quem gerencia quem
          </CardTitle>
          <CardDescription>
            Espelha <code className="font-mono text-xs">store.Role.CanManage</code>: rank do alvo ≤ rank do ator. Ninguém
            altera o próprio papel.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Ator</TableHead>
                {ranked.map((target) => (
                  <TableHead key={target} className="text-center">
                    {ROLE_LABELS[target]}
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {ranked.map((actor) => (
                <TableRow key={actor}>
                  <TableCell className="font-medium">{ROLE_LABELS[actor]}</TableCell>
                  {ranked.map((target) => {
                    const ok = canManageRole(actor, target)
                    return (
                      <TableCell key={target} className="text-center">
                        <span className={ok ? 'text-primary' : 'text-muted-foreground/40'}>{ok ? 'sim' : '—'}</span>
                      </TableCell>
                    )
                  })}
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {caller && (
            <p className="mt-3 text-sm text-muted-foreground">
              Seu papel ({ROLE_LABELS[caller.role]}) gerencia:{' '}
              {ranked
                .filter((r) => canManageRole(caller.role, r))
                .map((r) => ROLE_LABELS[r])
                .join(', ') || 'ninguém'}
              .
            </p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Permissões por papel</CardTitle>
          <CardDescription>
            O que cada papel alcança no painel e na API — ver PLAN.md §6.7. Um <code className="font-mono text-xs">admin</code>{' '}
            ainda pode ser limitado por <code className="font-mono text-xs">products: […]</code> (Fase 33): sem o produto na
            lista, a escrita daquela seção retorna 403. Lista vazia = irrestrito. <code className="font-mono text-xs">super_admin</code>{' '}
            ignora o escopo. IAM (usuários, papéis, auditoria) não é produto.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Capacidade</TableHead>
                {ranked.map((role) => (
                  <TableHead key={role} className="text-center">
                    {ROLE_LABELS[role]}
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {ROLE_CAPABILITIES.map((cap) => (
                <TableRow key={cap.id}>
                  <TableCell className="max-w-xs text-sm">{cap.label}</TableCell>
                  {ranked.map((role) => {
                    const ok = cap.roles.includes(role)
                    return (
                      <TableCell key={role} className="text-center">
                        <span className={ok ? 'text-primary' : 'text-muted-foreground/40'}>{ok ? 'sim' : '—'}</span>
                      </TableCell>
                    )
                  })}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}

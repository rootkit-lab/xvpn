import { useCallback, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { api, type User } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { formatDateTime } from '@/lib/format'
import { useAuth } from '@/lib/auth-context'
import { isAdminRole, ALL_ROLES, PRODUCT_LABELS, ROLE_BADGE_VARIANT, ROLE_LABELS, type Role } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { FilterBar } from '@/components/filter-bar'
import { DataTable, type DataTableColumn } from '@/components/data-table'

export function UsersPage() {
  const { user: caller } = useAuth()
  const navigate = useNavigate()
  const canCreate = isAdminRole(caller?.role)
  const [page, setPage] = useState(1)
  const [q, setQ] = useState('')
  const [roleFilter, setRoleFilter] = useState<Role | 'all'>('all')
  const [sftpFilter, setSftpFilter] = useState<'all' | 'on' | 'off'>('all')
  const [sambaFilter, setSambaFilter] = useState<'all' | 'on' | 'off'>('all')

  const fetchUsers = useCallback(
    () =>
      api.listUsers({
        page,
        per_page: 25,
        q,
        role: roleFilter === 'all' ? undefined : roleFilter,
        sftp: sftpFilter === 'all' ? undefined : sftpFilter === 'on' ? '1' : '0',
        samba: sambaFilter === 'all' ? undefined : sambaFilter === 'on' ? '1' : '0',
      }),
    [page, q, roleFilter, sftpFilter, sambaFilter],
  )
  const { data, loading } = usePollingData(fetchUsers, 30_000)

  const columns: DataTableColumn<User>[] = [
    { key: 'user', header: 'Usuário', cell: (u) => <span className="font-medium">{u.username}</span> },
    {
      key: 'role',
      header: 'Papel',
      cell: (u) => (
        <span className="flex flex-wrap items-center gap-1">
          <Badge variant={ROLE_BADGE_VARIANT[u.role]}>{ROLE_LABELS[u.role]}</Badge>
          {u.role === 'admin' && (u.products?.length ?? 0) > 0
            ? u.products!.map((p) => (
                <Badge key={p} variant="outline">
                  {PRODUCT_LABELS[p]}
                </Badge>
              ))
            : null}
        </span>
      ),
    },
    {
      key: 'files',
      header: 'Arquivos',
      cell: (u) => (
        <span className="flex flex-wrap gap-1">
          <Badge variant={u.sftp_enabled ? 'secondary' : 'outline'}>SFTP {u.sftp_enabled ? 'on' : 'off'}</Badge>
          <Badge variant={u.samba_enabled ? 'secondary' : 'outline'}>SMB {u.samba_enabled ? 'on' : 'off'}</Badge>
        </span>
      ),
    },
    {
      key: 'created',
      header: 'Criado em',
      cell: (u) => <span className="text-muted-foreground">{formatDateTime(u.created_at)}</span>,
    },
  ]

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <FilterBar
          q={q}
          onQChange={(next) => {
            setQ(next)
            setPage(1)
          }}
          placeholder="Buscar usuário"
        >
          <Button
            type="button"
            variant={sftpFilter === 'on' ? 'default' : 'outline'}
            size="sm"
            onClick={() => {
              setSftpFilter((v) => (v === 'on' ? 'all' : 'on'))
              setPage(1)
            }}
          >
            SFTP
          </Button>
          <Button
            type="button"
            variant={sambaFilter === 'on' ? 'default' : 'outline'}
            size="sm"
            onClick={() => {
              setSambaFilter((v) => (v === 'on' ? 'all' : 'on'))
              setPage(1)
            }}
          >
            Samba
          </Button>
        </FilterBar>
        {canCreate && (
          <Button onClick={() => navigate('/admin/users/new')}>
            <Plus className="size-4" />
            Novo usuário
          </Button>
        )}
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {ALL_ROLES.map((role) => {
          const active = roleFilter === role
          return (
            <button
              key={role}
              type="button"
              onClick={() => {
                setRoleFilter(active ? 'all' : role)
                setPage(1)
              }}
              className="text-left"
            >
              <Card className={active ? 'border-primary/50 bg-primary/10' : 'hover:border-primary/30'}>
                <CardHeader className="pb-2">
                  <CardTitle className="flex items-center justify-between text-sm font-medium">
                    <Badge variant={ROLE_BADGE_VARIANT[role]}>{ROLE_LABELS[role]}</Badge>
                    <span className="font-mono text-xs text-muted-foreground">{active ? 'filtro' : 'filtrar'}</span>
                  </CardTitle>
                </CardHeader>
              </Card>
            </button>
          )
        })}
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">
            {roleFilter === 'all' ? 'Todos os usuários' : ROLE_LABELS[roleFilter]}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <DataTable
            columns={columns}
            rows={data?.items ?? []}
            rowKey={(u) => u.id}
            loading={loading || !data}
            emptyTitle="Nenhum usuário neste filtro."
            onRowClick={(u) => navigate(`/admin/users/${u.id}`)}
            page={data?.page ?? page}
            perPage={data?.per_page ?? 25}
            total={data?.total ?? 0}
            onPageChange={setPage}
          />
        </CardContent>
      </Card>
    </div>
  )
}

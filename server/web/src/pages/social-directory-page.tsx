import { useCallback, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type SocialProfile } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { FilterBar } from '@/components/filter-bar'
import { DataTable, type DataTableColumn } from '@/components/data-table'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

export function SocialDirectoryPage() {
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const [q, setQ] = useState('')
  const fetchPeople = useCallback(() => api.listSocialPeople({ page, per_page: 25, q }), [page, q])
  const { data, loading } = usePollingData(fetchPeople, 20_000)

  const columns: DataTableColumn<SocialProfile>[] = [
    {
      key: 'name',
      header: 'Pessoa',
      cell: (p) => (
        <span className="font-medium">
          {p.display_name || p.username}{' '}
          <span className="text-muted-foreground">@{p.username}</span>
        </span>
      ),
    },
    {
      key: 'bio',
      header: 'Bio',
      cell: (p) => <span className="text-muted-foreground">{p.bio || '—'}</span>,
    },
    {
      key: 'follow',
      header: '',
      cell: (p) =>
        p.following ? (
          <Badge variant="secondary">seguindo</Badge>
        ) : (
          <span className="text-xs text-muted-foreground">{p.followers} seguidores</span>
        ),
    },
  ]

  return (
    <div className="flex flex-col gap-6">
      <FilterBar
        q={q}
        onQChange={(next) => {
          setQ(next)
          setPage(1)
        }}
        placeholder="Buscar pessoas"
      />
      <Card>
        <CardContent className="pt-6">
          <DataTable
            columns={columns}
            rows={data?.items ?? []}
            rowKey={(p) => p.user_id}
            loading={loading || !data}
            emptyTitle="Ninguém neste filtro."
            emptyDescription="Quando houver membros, eles aparecem aqui."
            onRowClick={(p) => navigate(`/social/u/${p.username}`)}
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

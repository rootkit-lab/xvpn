import { useCallback, useState } from 'react'
import { Link } from 'react-router-dom'
import { toast } from 'sonner'
import { api, ApiError, type SocialProfile } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { FilterBar } from '@/components/filter-bar'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'

export function SocialDirectoryPage() {
  const { user } = useAuth()
  const [page, setPage] = useState(1)
  const [q, setQ] = useState('')
  const fetchPeople = useCallback(() => api.listSocialPeople({ page, per_page: 24, q }), [page, q])
  const { data, loading, reload } = usePollingData(fetchPeople, 20_000)
  const pages = Math.max(1, Math.ceil((data?.total ?? 0) / (data?.per_page ?? 24)))

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-5">
      <FilterBar
        q={q}
        onQChange={(next) => {
          setQ(next)
          setPage(1)
        }}
        placeholder="Buscar pessoas"
      />
      {loading || !data ? (
        <Skeleton className="h-48 w-full rounded-[22px]" />
      ) : data.items.length === 0 ? (
        <p className="text-sm text-muted-foreground">Ninguém neste filtro.</p>
      ) : (
        <div className="flex flex-col">
          {data.items.map((p) => (
            <ExploreRow key={p.user_id} profile={p} isMe={p.username === user?.username} onChanged={reload} />
          ))}
        </div>
      )}
      {pages > 1 && (
        <div className="flex items-center justify-end gap-2 text-sm text-muted-foreground">
          <Button variant="ghost" size="sm" disabled={page <= 1} onClick={() => setPage((n) => n - 1)}>
            Anterior
          </Button>
          <span>
            {page}/{pages}
          </span>
          <Button variant="ghost" size="sm" disabled={page >= pages} onClick={() => setPage((n) => n + 1)}>
            Próxima
          </Button>
        </div>
      )}
    </div>
  )
}

function ExploreRow({
  profile,
  isMe,
  onChanged,
}: {
  profile: SocialProfile
  isMe: boolean
  onChanged: () => void
}) {
  const [busy, setBusy] = useState(false)

  async function toggle() {
    setBusy(true)
    try {
      if (profile.following) await api.unfollowUser(profile.username)
      else await api.followUser(profile.username)
      onChanged()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao atualizar follow')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex items-center gap-3 border-b border-white/8 py-3">
      <Link
        to={`/social/u/${profile.username}`}
        className="icon-well flex size-12 shrink-0 items-center justify-center rounded-full text-sm font-semibold"
      >
        {(profile.display_name || profile.username).slice(0, 1).toUpperCase()}
      </Link>
      <Link to={`/social/u/${profile.username}`} className="min-w-0 flex-1">
        <p className="truncate font-display text-sm font-semibold">{profile.display_name || profile.username}</p>
        <p className="truncate text-xs text-muted-foreground">@{profile.username}</p>
        {profile.bio && <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{profile.bio}</p>}
      </Link>
      {!isMe && (
        <Button
          size="sm"
          className="rounded-full"
          variant={profile.following ? 'outline' : 'default'}
          disabled={busy}
          onClick={toggle}
        >
          {profile.following ? 'Seguindo' : 'Seguir'}
        </Button>
      )}
    </div>
  )
}

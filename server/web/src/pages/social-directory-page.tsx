import { useCallback, useState } from 'react'
import { ProfileLink } from '@/components/profile-link'
import { Search } from 'lucide-react'
import { toast } from 'sonner'
import { api, ApiError, type SocialProfile } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { FilterBar } from '@/components/filter-bar'
import { SocialAvatar } from '@/components/social-avatar'
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
    <div className="flex w-full min-w-0 flex-col gap-6">
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
        <div className="watch-complication flex flex-col items-center gap-2 rounded-[22px] px-5 py-12 text-center">
          <Search className="size-5 text-muted-foreground" />
          <p className="font-display text-sm font-semibold">Ninguém neste filtro</p>
          <p className="text-sm text-muted-foreground">Tente outro nome ou limpe a busca.</p>
        </div>
      ) : (
        <ul className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {data.items.map((p) => (
            <li key={p.user_id}>
              <ExploreRow profile={p} isMe={p.username === user?.username} onChanged={reload} />
            </li>
          ))}
        </ul>
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
  const display = profile.display_name || profile.username

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
    <div className="watch-complication flex items-center gap-3 rounded-[18px] p-3.5">
      <ProfileLink username={profile.username} className="shrink-0">
        <SocialAvatar name={display} src={profile.avatar_url} className="size-12 text-sm" />
      </ProfileLink>
      <ProfileLink username={profile.username} className="min-w-0 flex-1">
        <p className="truncate font-display text-sm font-semibold">{display}</p>
        <p className="truncate text-xs text-muted-foreground">@{profile.username}</p>
        {profile.bio && <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">{profile.bio}</p>}
      </ProfileLink>
      {isMe ? (
        <span className="rounded-full border border-white/12 px-2.5 py-1 text-[11px] text-muted-foreground">
          Você
        </span>
      ) : (
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

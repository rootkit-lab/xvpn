import { useCallback, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { ProfileLink } from '@/components/profile-link'
import { SocialAvatar } from '@/components/social-avatar'
import { SocialStoriesRail } from '@/components/social-stories'
import { toast } from 'sonner'
import { api, ApiError, type SocialPost, type SocialProfile } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { useOptionalChat } from '@chat/messenger/ChatProvider'
import { livePresence } from '@/lib/social-presence'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Skeleton } from '@/components/ui/skeleton'
import { formatRelativeTime } from '@/lib/format'

export function SocialFeedPage() {
  const { user } = useAuth()
  const fetchFeed = useCallback(() => api.listSocialFeed({ page: 1, per_page: 40 }), [])
  const fetchPeople = useCallback(() => api.listSocialPeople({ page: 1, per_page: 6 }), [])
  const fetchGroups = useCallback(() => api.listSocialGroups({ page: 1, per_page: 4 }), [])
  const { data, loading, error, reload } = usePollingData(fetchFeed, 15_000)
  const { data: people } = usePollingData(fetchPeople, 30_000)
  const { data: groups } = usePollingData(fetchGroups, 30_000)
  const [body, setBody] = useState('')
  const [posting, setPosting] = useState(false)
  const left = 280 - [...body].length

  async function publish(e: FormEvent) {
    e.preventDefault()
    const text = body.trim()
    if (!text) return
    setPosting(true)
    try {
      await api.createSocialPost(text)
      setBody('')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao publicar')
    } finally {
      setPosting(false)
    }
  }

  const suggestions = (people?.items ?? []).filter((p) => p.username !== user?.username && !p.following).slice(0, 4)

  return (
    <div className="flex w-full min-w-0 flex-col gap-5">
      <SocialStoriesRail />

      <div className="grid w-full min-w-0 gap-6 xl:grid-cols-[minmax(0,1fr)_20rem]">
        <div className="flex flex-col gap-4">
          <form onSubmit={publish} className="watch-complication rounded-[22px] p-4">
            <Textarea
              value={body}
              onChange={(e) => setBody(e.target.value.slice(0, 280))}
              placeholder="O que está acontecendo?"
              rows={3}
            />
            <div className="mt-3 flex items-center justify-between">
              <span className={`text-xs ${left < 20 ? 'text-destructive' : 'text-muted-foreground'}`}>{left}</span>
              <Button type="submit" size="lg" className="rounded-full" disabled={posting || !body.trim()}>
                {posting ? 'Publicando…' : 'Postar'}
              </Button>
            </div>
          </form>

          {error && <p className="text-sm text-destructive">{error}</p>}
          {loading || !data ? (
            <Skeleton className="h-40 w-full rounded-[22px]" />
          ) : data.items.length === 0 ? (
            <p className="text-sm text-muted-foreground">Ainda não há posts. Seja o primeiro.</p>
          ) : (
            <div className="flex flex-col gap-3">
              {data.items.map((post) => (
                <PostCard key={post.id} post={post} />
              ))}
            </div>
          )}
        </div>

        <aside className="hidden xl:flex xl:flex-col xl:gap-4">
          <div className="watch-complication rounded-[22px] p-4">
            <p className="hud-label text-muted-foreground/70">Quem seguir</p>
            <div className="mt-3 flex flex-col gap-3">
              {suggestions.map((p) => (
                <FollowHint key={p.user_id} profile={p} onChanged={reload} />
              ))}
              {suggestions.length === 0 && <p className="text-xs text-muted-foreground">Você já segue todo mundo.</p>}
            </div>
            <Link to="/social/explore" className="mt-4 inline-block text-sm text-primary hover:underline">
              Mostrar mais
            </Link>
          </div>
          <div className="watch-complication rounded-[22px] p-4">
            <p className="hud-label text-muted-foreground/70">Grupos</p>
            <div className="mt-3 flex flex-col gap-2">
              {(groups?.items ?? []).slice(0, 4).map((g) => (
                <Link key={g.id} to="/social/groups" className="rounded-[14px] px-2 py-1.5 hover:bg-white/6">
                  <p className="truncate text-sm font-semibold">{g.name}</p>
                  <p className="truncate text-xs text-muted-foreground">
                    {g.member_count} membro{g.member_count === 1 ? '' : 's'}
                  </p>
                </Link>
              ))}
              {(groups?.items ?? []).length === 0 && (
                <p className="text-xs text-muted-foreground">Nenhum grupo ainda.</p>
              )}
            </div>
            <Link to="/social/groups" className="mt-3 inline-block text-sm text-primary hover:underline">
              Ver grupos
            </Link>
          </div>
        </aside>
      </div>
    </div>
  )
}

export function PostCard({ post }: { post: SocialPost }) {
  const chat = useOptionalChat()
  const presence = livePresence(post.author_id, post.presence, chat?.presence)
  const display = post.display_name || post.username

  return (
    <article className="watch-complication flex gap-3 rounded-[22px] px-4 py-4">
      <ProfileLink username={post.username} className="shrink-0">
        <SocialAvatar name={display} presence={presence} className="size-11 text-sm" />
      </ProfileLink>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-baseline gap-x-2">
          <ProfileLink username={post.username} className="font-display text-sm font-semibold hover:underline">
            {display}
          </ProfileLink>
          <span className="text-xs text-muted-foreground">@{post.username}</span>
          <span className="text-xs text-muted-foreground">· {formatRelativeTime(post.created_at)}</span>
        </div>
        <p className="mt-1.5 whitespace-pre-wrap text-sm leading-relaxed">{post.body}</p>
      </div>
    </article>
  )
}

function FollowHint({ profile, onChanged }: { profile: SocialProfile; onChanged: () => void }) {
  const chat = useOptionalChat()
  const [busy, setBusy] = useState(false)
  const presence = livePresence(profile.user_id, profile.presence, chat?.presence)

  async function follow() {
    setBusy(true)
    try {
      await api.followUser(profile.username)
      onChanged()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao seguir')
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="flex items-center gap-2">
      <SocialAvatar
        name={profile.display_name || profile.username}
        presence={presence}
        className="size-9 text-xs"
      />
      <ProfileLink username={profile.username} className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold">{profile.display_name || profile.username}</p>
        <p className="truncate text-xs text-muted-foreground">@{profile.username}</p>
      </ProfileLink>
      <Button size="sm" className="rounded-full" disabled={busy} onClick={follow}>
        Seguir
      </Button>
    </div>
  )
}

import { useCallback, useState, type FormEvent } from 'react'
import { Navigate, useParams } from 'react-router-dom'
import { isProfileUsername } from '@/lib/social-profile'
import { MessageCircle, UserPlus, UserMinus } from 'lucide-react'
import { openChat } from '@chat/messenger/open-chat'
import { toast } from 'sonner'
import { api, ApiError, type SocialProfile } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { PostCard } from '@/pages/social-feed-page'
import { SocialAvatar } from '@/components/social-avatar'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Skeleton } from '@/components/ui/skeleton'

/** `/:username` no host público — só se o slug for um membro. */
export function SocialProfileGate() {
  const { username } = useParams()
  if (!username || !isProfileUsername(username)) {
    return <Navigate to="/" replace />
  }
  return <SocialProfilePage />
}

export function SocialProfilePage() {
  const { username } = useParams()
  const { user } = useAuth()
  const fetchProfile = useCallback(() => api.getSocialProfile(username ?? ''), [username])
  const fetchPosts = useCallback(
    () => api.listSocialUserPosts(username ?? '', { page: 1, per_page: 40 }),
    [username],
  )
  const { data, loading, error, reload } = usePollingData(fetchProfile, 20_000)
  const { data: posts, loading: postsLoading } = usePollingData(fetchPosts, 15_000)
  const [editing, setEditing] = useState(false)

  if (!username) return <p className="text-sm text-destructive">Usuário inválido.</p>
  if (loading || !data) {
    return error ? (
      <p className="text-sm text-destructive">{error}</p>
    ) : (
      <Skeleton className="h-72 w-full rounded-[22px]" />
    )
  }

  const profile = data
  const isMe = user?.username === profile.username
  const display = profile.display_name || profile.username

  async function toggleFollow() {
    try {
      if (profile.following) await api.unfollowUser(profile.username)
      else await api.followUser(profile.username)
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao atualizar follow')
    }
  }

  return (
    <div className="flex w-full min-w-0 flex-col gap-6">
      <section className="overflow-hidden rounded-[22px] watch-complication">
        <div className={`h-36 w-full md:h-48 ${bannerClass(profile.username)}`} />
        <div className="px-5 pb-6 md:px-8">
          <div className="-mt-14 flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div className="flex min-w-0 flex-col gap-4 sm:flex-row sm:items-end">
              <SocialAvatar
                name={display}
                className="size-28 shrink-0 border-4 border-background text-3xl shadow-lg md:size-32"
              />
              <div className="min-w-0 pb-1">
                <div className="flex flex-wrap items-center gap-2">
                  <h1 className="font-display text-3xl font-semibold tracking-tight">{display}</h1>
                  {isMe && (
                    <span className="rounded-full border border-white/12 px-2 py-0.5 text-[10px] font-medium text-muted-foreground">
                      Você
                    </span>
                  )}
                </div>
                <p className="mt-0.5 text-sm text-muted-foreground">@{profile.username}</p>
              </div>
            </div>
            <div className="flex flex-wrap gap-2 lg:mb-1">
              {isMe ? (
                <Button variant="outline" className="rounded-full" onClick={() => setEditing((v) => !v)}>
                  {editing ? 'Fechar' : 'Editar perfil'}
                </Button>
              ) : (
                <>
                  <Button
                    variant={profile.following ? 'outline' : 'default'}
                    className="rounded-full"
                    onClick={toggleFollow}
                  >
                    {profile.following ? <UserMinus className="size-4" /> : <UserPlus className="size-4" />}
                    {profile.following ? 'Seguindo' : 'Seguir'}
                  </Button>
                  <Button
                    variant="secondary"
                    className="rounded-full"
                    onClick={() => openChat({ username: profile.username })}
                  >
                    <MessageCircle className="size-4" />
                    Mensagem
                  </Button>
                </>
              )}
            </div>
          </div>

          <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-start">
            <div className="min-w-0">
              {profile.bio ? (
                <p className="max-w-3xl text-sm leading-relaxed">{profile.bio}</p>
              ) : isMe ? (
                <p className="text-sm text-muted-foreground">Sem bio ainda. Edite o perfil para se apresentar.</p>
              ) : null}
            </div>
            <dl className="flex flex-wrap gap-2">
              <div className="rounded-full bg-white/6 px-3 py-1.5 text-sm">
                <dt className="sr-only">Seguindo</dt>
                <dd>
                  <span className="font-semibold text-foreground">{profile.following_count ?? 0}</span>
                  <span className="ml-1 text-muted-foreground">seguindo</span>
                </dd>
              </div>
              <div className="rounded-full bg-white/6 px-3 py-1.5 text-sm">
                <dt className="sr-only">Seguidores</dt>
                <dd>
                  <span className="font-semibold text-foreground">{profile.followers}</span>
                  <span className="ml-1 text-muted-foreground">
                    seguidor{profile.followers === 1 ? '' : 'es'}
                  </span>
                </dd>
              </div>
            </dl>
          </div>
        </div>
      </section>

      {isMe && editing && (
        <EditSocialProfile
          profile={profile}
          onSaved={() => {
            setEditing(false)
            reload()
          }}
        />
      )}

      <section>
        <p className="hud-label mb-3 text-muted-foreground/70">Atividade</p>
        {postsLoading || !posts ? (
          <Skeleton className="h-32 w-full rounded-[22px]" />
        ) : posts.items.length === 0 ? (
          <div className="watch-complication rounded-[22px] px-5 py-10 text-center">
            <p className="font-display text-sm font-semibold">Ainda sem posts</p>
            <p className="mt-1 text-sm text-muted-foreground">
              {isMe ? 'Publique no início para aparecer aqui.' : `${display} ainda não publicou.`}
            </p>
          </div>
        ) : (
          <div className="watch-complication divide-y divide-white/8 overflow-hidden rounded-[22px] px-4 md:px-6">
            {posts.items.map((post) => (
              <PostCard key={post.id} post={post} />
            ))}
          </div>
        )}
      </section>
    </div>
  )
}

function bannerClass(username: string): string {
  const n = [...username].reduce((acc, ch) => acc + ch.charCodeAt(0), 0)
  const tones = ['bg-primary/35', 'bg-chart-2/40', 'bg-chart-3/40', 'bg-chart-4/35', 'bg-chart-5/35']
  return tones[n % tones.length]
}

function EditSocialProfile({ profile, onSaved }: { profile: SocialProfile; onSaved: () => void }) {
  const [displayName, setDisplayName] = useState(profile.display_name)
  const [bio, setBio] = useState(profile.bio)
  const [saving, setSaving] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSaving(true)
    try {
      await api.patchSocialMe({ display_name: displayName, bio })
      toast.success('Perfil atualizado')
      onSaved()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao salvar perfil')
    } finally {
      setSaving(false)
    }
  }

  return (
    <form className="watch-complication grid gap-4 rounded-[22px] p-5 md:grid-cols-2 md:p-6" onSubmit={handleSubmit}>
      <p className="hud-label text-muted-foreground/70 md:col-span-2">Editar</p>
      <div className="flex flex-col gap-2">
        <Label htmlFor="display-name">Nome</Label>
        <Input id="display-name" value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
      </div>
      <div className="flex flex-col gap-2 md:col-span-2">
        <Label htmlFor="bio">Bio</Label>
        <Textarea id="bio" value={bio} onChange={(e) => setBio(e.target.value)} rows={3} />
      </div>
      <Button type="submit" className="self-start rounded-full" disabled={saving}>
        {saving ? 'Salvando…' : 'Salvar'}
      </Button>
    </form>
  )
}

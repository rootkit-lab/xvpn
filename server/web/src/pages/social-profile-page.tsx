import { useCallback, useState, type FormEvent } from 'react'
import { useParams } from 'react-router-dom'
import { openChat } from '@chat/messenger/open-chat'
import { toast } from 'sonner'
import { api, ApiError, type SocialProfile } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { PostCard } from '@/pages/social-feed-page'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Skeleton } from '@/components/ui/skeleton'

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
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-64 w-full rounded-[22px]" />
  }

  const profile = data
  const isMe = user?.username === profile.username

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
    <div className="mx-auto flex w-full max-w-2xl flex-col">
      <div className="overflow-hidden rounded-[22px] watch-complication">
        <div className={`h-36 w-full ${bannerClass(profile.username)}`} />
        <div className="px-5 pb-5">
          <div className="-mt-10 flex items-end justify-between gap-3">
            <div className="icon-well flex size-20 items-center justify-center rounded-full border-4 border-background text-2xl font-semibold">
              {(profile.display_name || profile.username).slice(0, 1).toUpperCase()}
            </div>
            <div className="mb-1 flex flex-wrap gap-2">
              {isMe ? (
                <Button variant="outline" className="rounded-full" onClick={() => setEditing((v) => !v)}>
                  {editing ? 'Fechar' : 'Editar perfil'}
                </Button>
              ) : (
                <>
                  <Button variant={profile.following ? 'outline' : 'default'} className="rounded-full" onClick={toggleFollow}>
                    {profile.following ? 'Seguindo' : 'Seguir'}
                  </Button>
                  <Button variant="secondary" className="rounded-full" onClick={() => openChat({ username: profile.username })}>
                    Mensagem
                  </Button>
                </>
              )}
            </div>
          </div>
          <h1 className="font-display mt-3 text-xl font-semibold">{profile.display_name || profile.username}</h1>
          <p className="text-sm text-muted-foreground">@{profile.username}</p>
          {profile.bio && <p className="mt-3 text-sm leading-relaxed">{profile.bio}</p>}
          <p className="mt-3 text-sm text-muted-foreground">
            <span className="font-semibold text-foreground">{profile.following_count ?? 0}</span> seguindo
            <span className="mx-2">·</span>
            <span className="font-semibold text-foreground">{profile.followers}</span> seguidor
            {profile.followers === 1 ? '' : 'es'}
          </p>
        </div>
      </div>

      {isMe && editing && (
        <div className="mt-4">
          <EditSocialProfile
            profile={profile}
            onSaved={() => {
              setEditing(false)
              reload()
            }}
          />
        </div>
      )}

      <div className="mt-2">
        {postsLoading || !posts ? (
          <Skeleton className="mt-4 h-32 w-full rounded-[22px]" />
        ) : posts.items.length === 0 ? (
          <p className="mt-6 text-sm text-muted-foreground">Ainda sem posts.</p>
        ) : (
          posts.items.map((post) => <PostCard key={post.id} post={post} />)
        )}
      </div>
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
    <form className="watch-complication flex flex-col gap-4 rounded-[22px] p-5" onSubmit={handleSubmit}>
      <div className="flex flex-col gap-2">
        <Label htmlFor="display-name">Nome</Label>
        <Input id="display-name" value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor="bio">Bio</Label>
        <Textarea id="bio" value={bio} onChange={(e) => setBio(e.target.value)} rows={3} />
      </div>
      <Button type="submit" className="rounded-full self-start" disabled={saving}>
        {saving ? 'Salvando…' : 'Salvar'}
      </Button>
    </form>
  )
}

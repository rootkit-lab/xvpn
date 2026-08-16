import { useCallback, useState, type FormEvent } from 'react'
import { useParams } from 'react-router-dom'
import { openChat } from '@chat/messenger/open-chat'
import { toast } from 'sonner'
import { api, ApiError, type SocialProfile } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'

export function SocialProfilePage() {
  const { username } = useParams()
  const { user } = useAuth()
  const fetchProfile = useCallback(() => api.getSocialProfile(username ?? ''), [username])
  const { data, loading, error, reload } = usePollingData(fetchProfile, 20_000)

  if (!username) return <p className="text-sm text-destructive">Usuário inválido.</p>
  if (loading || !data) {
    return error ? <p className="text-sm text-destructive">{error}</p> : <Skeleton className="h-48 w-full" />
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

  function openDM() {
    openChat({ username: profile.username })
  }

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-xl">{profile.display_name || profile.username}</CardTitle>
          <CardDescription>
            @{profile.username} · {profile.followers} seguidor{profile.followers === 1 ? '' : 'es'}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {profile.bio && <p className="text-sm">{profile.bio}</p>}
          <div className="flex flex-wrap gap-2">
            {isMe ? (
              <Badge variant="secondary">você</Badge>
            ) : (
              <>
                <Button variant={profile.following ? 'outline' : 'default'} onClick={toggleFollow}>
                  {profile.following ? 'Deixar de seguir' : 'Seguir'}
                </Button>
                <Button variant="secondary" onClick={openDM}>
                  Mensagem
                </Button>
              </>
            )}
          </div>
        </CardContent>
      </Card>
      {isMe && <EditSocialProfile profile={profile} onSaved={reload} />}
    </div>
  )
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
      toast.success('Perfil social atualizado')
      onSaved()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao salvar perfil')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Editar perfil social</CardTitle>
        <CardDescription>Isso não altera senha nem SSH — isso fica em xvpn → Conta.</CardDescription>
      </CardHeader>
      <CardContent>
        <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
          <div className="flex flex-col gap-2">
            <Label htmlFor="display-name">Nome de exibição</Label>
            <Input id="display-name" value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="bio">Bio</Label>
            <Textarea id="bio" value={bio} onChange={(e) => setBio(e.target.value)} rows={4} />
          </div>
          <Button type="submit" disabled={saving}>
            {saving ? 'Salvando…' : 'Salvar'}
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}

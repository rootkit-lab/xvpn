import { useCallback, useState } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import { AtSign, Mail, MessageCircle, Package, Star } from 'lucide-react'
import { toast } from 'sonner'
import { api, ApiError } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { useOptionalChat } from '@chat/messenger/ChatProvider'
import { XCHAT_CORP_ORIGIN, XGROUP_ORIGIN } from '@/lib/product-host'
import { SocialAvatar } from '@/components/social-avatar'
import { EditSocialProfileDialog } from '@/components/edit-social-profile-dialog'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

const TABS: { to: string; label: string; end?: boolean; count?: 'repos' | 'stars' }[] = [
  { to: '/', label: 'Overview', end: true },
  { to: '/repositories', label: 'Repositórios', count: 'repos' },
  { to: '/packages', label: 'Packages' },
  { to: '/stars', label: 'Stars', count: 'stars' },
]

export function XgitHomeLayout() {
  const { user } = useAuth()
  const chat = useOptionalChat()
  const fetchOverview = useCallback(() => api.getXgitOverview(), [])
  const fetchMe = useCallback(() => api.getSocialMe(), [])
  const { data: overview } = usePollingData(fetchOverview, 30_000)
  const { data: me, reload } = usePollingData(fetchMe, 30_000)
  const [editing, setEditing] = useState(false)
  const profile = me ?? overview?.profile
  const display = profile?.display_name || user?.username || ''

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-col gap-6">
      <nav className="flex flex-wrap gap-1 border-b border-border/60">
        {TABS.map((tab) => (
          <NavLink
            key={tab.to}
            to={tab.to}
            end={tab.end}
            className={({ isActive }) =>
              cn(
                'border-b-2 px-3 py-2 text-sm',
                isActive
                  ? 'border-primary text-foreground'
                  : 'border-transparent text-muted-foreground hover:text-foreground',
              )
            }
          >
            {tab.label}
            {tab.count === 'repos' && overview ? (
              <span className="ml-1.5 text-xs text-muted-foreground">{overview.repo_count}</span>
            ) : null}
            {tab.count === 'stars' && overview ? (
              <span className="ml-1.5 text-xs text-muted-foreground">{overview.star_count}</span>
            ) : null}
          </NavLink>
        ))}
      </nav>

      <div className="grid gap-8 lg:grid-cols-[16rem_minmax(0,1fr)]">
        <aside className="flex flex-col items-start gap-3">
          <SocialAvatar
            name={display}
            src={profile?.avatar_url}
            presence={profile?.presence}
            className="size-40 text-4xl"
            textClassName="text-4xl"
          />
          <div>
            <h1 className="font-display text-2xl font-semibold tracking-tight">{display}</h1>
            <p className="text-sm text-muted-foreground">{user?.username}</p>
          </div>
          {profile?.bio ? <p className="text-sm text-muted-foreground">{profile.bio}</p> : null}
          <Button type="button" variant="outline" size="sm" className="w-full" onClick={() => setEditing(true)}>
            Editar perfil
          </Button>
          <div className="flex flex-col gap-1.5 text-sm text-muted-foreground">
            <a
              href={`${XGROUP_ORIGIN}/${user?.username ?? ''}`}
              className="inline-flex items-center gap-2 hover:text-foreground"
            >
              <AtSign className="size-3.5" />
              Perfil XGROUP
            </a>
            {chat ? (
              <button
                type="button"
                className="inline-flex items-center gap-2 text-left hover:text-foreground"
                onClick={() => chat.setDockOpen(true)}
              >
                <MessageCircle className="size-3.5" />
                Abrir XCHAT
              </button>
            ) : (
              <a href={`${XCHAT_CORP_ORIGIN}/social/messages`} className="inline-flex items-center gap-2 hover:text-foreground">
                <Mail className="size-3.5" />
                Mensagens
              </a>
            )}
            <span className="inline-flex items-center gap-2">
              <Package className="size-3.5" />
              Packages em breve
            </span>
            <span className="inline-flex items-center gap-2">
              <Star className="size-3.5" />
              {overview?.star_count ?? 0} stars
            </span>
          </div>
        </aside>
        <div className="min-w-0">
          <Outlet />
        </div>
      </div>

      {profile ? (
        <EditSocialProfileDialog
          open={editing}
          onOpenChange={setEditing}
          profile={profile}
          onSaved={async () => {
            try {
              await reload()
            } catch (err) {
              toast.error(err instanceof ApiError ? err.message : 'Falha ao recarregar perfil')
            }
          }}
        />
      ) : null}
    </div>
  )
}

import { useNavigate } from 'react-router-dom'
import { Settings } from 'lucide-react'
import { IconButton } from '@xvpn/ui/react/icon-button'
import { useChatSettings } from '@chat/messenger/ChatSettings'
import { PANEL_ORIGIN, XADMIN_CORP_ORIGIN, productKind } from '@/lib/product-host'

export type AppSettingsKind = 'user' | 'admin' | 'social' | 'marketplace' | 'xdriver' | 'xgit'

/** Prefs do app atual — não é a conta (isso fica no AccountMenu). */
export function AppSettingsButton({ kind }: { kind: AppSettingsKind }) {
  if (kind === 'social') return <SocialAppSettingsButton />
  return <LinkedAppSettingsButton kind={kind} />
}

function LinkedAppSettingsButton({ kind }: { kind: Exclude<AppSettingsKind, 'social'> }) {
  const navigate = useNavigate()
  const to = kind === 'admin' ? '/admin/settings' : kind === 'user' ? '/my/account' : '/settings'
  const offPanel = productKind() !== 'xvpn' && (kind === 'admin' || kind === 'user')
  if (kind === 'xgit') {
    return (
      <IconButton
        label="Configurações"
        filled
        onClick={() => {
          window.location.assign(`${XADMIN_CORP_ORIGIN}/admin/xgit/settings`)
        }}
      >
        <Settings className="size-4" strokeWidth={2} />
      </IconButton>
    )
  }
  return (
    <IconButton
      label="Configurações"
      filled
      onClick={() => {
        if (offPanel) {
          window.location.assign(`${PANEL_ORIGIN}${to}`)
          return
        }
        navigate(to)
      }}
    >
      <Settings className="size-4" strokeWidth={2} />
    </IconButton>
  )
}

function SocialAppSettingsButton() {
  const { setOpen } = useChatSettings()
  return (
    <IconButton label="Configurações" filled onClick={() => setOpen(true)}>
      <Settings className="size-4" strokeWidth={2} />
    </IconButton>
  )
}

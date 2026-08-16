import { useState } from 'react'
import { LogOut, Palette, Settings } from 'lucide-react'
import { useChat } from '@chat/messenger/ChatProvider'
import { ChatDropdown, ChatIconButton, ChatMenuItem, ChatShell } from '@chat/messenger/chrome'
import { ContactList } from '@chat/messenger/ContactList'
import { Conversation } from '@chat/messenger/Conversation'
import { NewChatDialog } from '@chat/messenger/NewChatDialog'
import { StatusDot } from '@chat/messenger/StatusDot'
import { StoriesRail } from '@chat/messenger/Stories'
import { ChatRoot } from '@chat/messenger/ui'
import { useChatSettings } from '@chat/messenger/ChatSettings'
import { useChatTheme } from '@chat/theme/ThemeProvider'
import { cn } from '@chat/lib/utils'
import type { ChatTheme, PresenceStatus } from '@chat/chatapi/types'

const STATUSES: Exclude<PresenceStatus, 'offline'>[] = ['online', 'away', 'dnd', 'invisible']
const THEMES: Exclude<ChatTheme, 'inherit'>[] = ['dark', 'light', 'icq']

const STATUS_LABEL: Record<string, string> = {
  online: 'Online',
  away: 'Ausente',
  dnd: 'Ocupado',
  invisible: 'Invisível',
}

const THEME_LABEL: Record<string, string> = {
  dark: 'Escuro',
  light: 'Claro',
  icq: 'ICQ',
}

export function Messenger({ className }: { className?: string }) {
  const { session, myStatus, setMyStatus, setActiveKey, logout, mode, activeKey } = useChat()
  const { theme, setTheme } = useChatTheme()
  const { setOpen: setSettingsOpen } = useChatSettings()
  const [picker, setPicker] = useState<'status' | 'theme' | null>(null)
  const desktop = mode === 'desktop'
  const resolvedTheme = desktop ? theme : 'inherit'

  const chrome = (
    <>
      {desktop && (
        <header className="relative z-10 mb-3 flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <img
              src="/logo-192.png"
              alt=""
              className="size-7 rounded-[9px] shadow-[inset_0_1px_0_color-mix(in_oklch,white_20%,transparent)]"
            />
            <div className="flex min-w-0 flex-col">
              <span className="font-display text-[17px] font-semibold tracking-tight">XCHAT</span>
              <span className="hud-label text-muted-foreground/70">Client</span>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <div className="relative">
              <ChatIconButton
                onClick={() => setPicker((p) => (p === 'status' ? null : 'status'))}
                label="Status"
                title={STATUS_LABEL[myStatus]}
                filled
              >
                <StatusDot status={myStatus === 'invisible' ? 'offline' : myStatus} className="ring-0" />
              </ChatIconButton>
              <ChatDropdown open={picker === 'status'} onClose={() => setPicker(null)}>
                {STATUSES.map((s) => (
                  <ChatMenuItem
                    key={s}
                    active={myStatus === s}
                    onClick={() => {
                      setMyStatus(s)
                      setPicker(null)
                    }}
                  >
                    <StatusDot status={s === 'invisible' ? 'offline' : s} className="ring-0" />
                    {STATUS_LABEL[s]}
                  </ChatMenuItem>
                ))}
              </ChatDropdown>
            </div>
            <div className="relative">
              <ChatIconButton
                onClick={() => setPicker((p) => (p === 'theme' ? null : 'theme'))}
                label="Tema"
                filled
              >
                <Palette className="h-4 w-4" strokeWidth={2} />
              </ChatIconButton>
              <ChatDropdown open={picker === 'theme'} onClose={() => setPicker(null)}>
                {THEMES.map((t) => (
                  <ChatMenuItem
                    key={t}
                    active={theme === t}
                    onClick={() => {
                      setTheme(t)
                      setPicker(null)
                    }}
                  >
                    {THEME_LABEL[t]}
                  </ChatMenuItem>
                ))}
              </ChatDropdown>
            </div>
            <ChatIconButton onClick={() => setSettingsOpen(true)} label="Configurações" filled>
              <Settings className="h-4 w-4" strokeWidth={2} />
            </ChatIconButton>
            <ChatIconButton onClick={() => void logout()} label="Sair" filled>
              <LogOut className="h-4 w-4" strokeWidth={2} />
            </ChatIconButton>
          </div>
        </header>
      )}
      <div className={cn('relative z-10 flex min-h-0 flex-1', desktop && 'gap-3')}>
        <aside
          className={cn(
            'flex w-72 shrink-0 flex-col',
            desktop ? 'overflow-hidden rounded-[22px] watch-complication' : 'border-r border-border bg-card',
          )}
        >
          {!desktop && (
            <div className="relative flex items-center gap-2 border-b border-border px-3 py-2">
              <button
                type="button"
                className="flex min-w-0 flex-1 items-center gap-2 rounded-[14px] px-1 py-1 text-left hover:bg-white/8"
                onClick={() => setPicker((p) => (p === 'status' ? null : 'status'))}
                aria-haspopup="menu"
                aria-expanded={picker === 'status'}
              >
                <StatusDot status={myStatus === 'invisible' ? 'offline' : myStatus} />
                <span className="truncate text-sm font-medium">{session?.username}</span>
              </button>
              <ChatDropdown open={picker === 'status'} onClose={() => setPicker(null)} align="left">
                {STATUSES.map((s) => (
                  <ChatMenuItem
                    key={s}
                    active={myStatus === s}
                    onClick={() => {
                      setMyStatus(s)
                      setPicker(null)
                    }}
                  >
                    <StatusDot status={s === 'invisible' ? 'offline' : s} className="ring-0" />
                    {STATUS_LABEL[s]}
                  </ChatMenuItem>
                ))}
              </ChatDropdown>
              <ChatIconButton onClick={() => setSettingsOpen(true)} label="Configurações">
                <Settings className="h-4 w-4" strokeWidth={2} />
              </ChatIconButton>
            </div>
          )}
          <StoriesRail />
          <NewChatDialog />
          <div className="min-h-0 flex-1">
            <ContactList onSelect={(k) => setActiveKey(k)} />
          </div>
        </aside>
        <section
          className={cn(
            'min-w-0 flex-1 bg-background',
            desktop && 'overflow-hidden rounded-[22px] watch-complication bg-transparent',
          )}
        >
          <Conversation threadKey={activeKey ?? ''} />
        </section>
      </div>
    </>
  )

  return (
    <ChatRoot theme={resolvedTheme} className={className ?? 'flex h-full min-h-0 overflow-hidden'}>
      {desktop ? <ChatShell className="min-h-0 flex-1">{chrome}</ChatShell> : chrome}
    </ChatRoot>
  )
}

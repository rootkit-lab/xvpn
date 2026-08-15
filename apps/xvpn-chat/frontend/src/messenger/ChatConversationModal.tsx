import { useEffect } from 'react'
import { useChat } from '@chat/messenger/ChatProvider'
import { Conversation } from '@chat/messenger/Conversation'
import { ChatRoot } from '@chat/messenger/ui'

/** Modal da conversa — a lista de contatos permanece na sidebar direita. */
export function ChatConversationModal() {
  const { session, activeKey, setActiveKey } = useChat()

  useEffect(() => {
    if (!activeKey) return
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setActiveKey(null)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [activeKey, setActiveKey])

  if (!session?.loggedIn || !activeKey) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <button
        type="button"
        className="absolute inset-0 bg-black/50"
        aria-label="Fechar conversa"
        onClick={() => setActiveKey(null)}
      />
      <ChatRoot
        theme="inherit"
        className="relative z-10 flex h-[min(36rem,85svh)] w-full max-w-lg flex-col overflow-hidden rounded-xl border border-border bg-card shadow-2xl"
      >
        <div
          role="dialog"
          aria-modal="true"
          aria-label="Conversa"
          className="flex h-full min-h-0 flex-col"
        >
          <Conversation threadKey={activeKey} onClose={() => setActiveKey(null)} />
        </div>
      </ChatRoot>
    </div>
  )
}

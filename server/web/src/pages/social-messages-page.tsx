import { Messenger } from '@chat/messenger/Messenger'

export function SocialMessagesPage() {
  return (
    <div className="flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <Messenger className="flex h-full min-h-0 min-w-0 overflow-hidden" />
    </div>
  )
}

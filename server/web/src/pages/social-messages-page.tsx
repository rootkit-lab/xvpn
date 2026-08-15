import { Messenger } from '@chat/messenger/Messenger'

export function SocialMessagesPage() {
  return (
    <div className="-mx-6 -mb-6 h-[calc(100svh-7.5rem)] md:-mx-8 md:-mb-8">
      <Messenger className="flex h-full min-h-0 overflow-hidden" />
    </div>
  )
}

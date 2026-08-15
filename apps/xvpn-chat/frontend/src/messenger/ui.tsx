import { cn } from '@chat/lib/utils'
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode } from 'react'

export function ChatButton({
  children,
  className,
  variant = 'default',
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: 'default' | 'ghost' | 'outline' }) {
  return (
    <button
      type="button"
      className={cn(
        'inline-flex h-9 items-center justify-center gap-1.5 rounded-md px-3 text-sm font-medium transition-colors disabled:opacity-50',
        variant === 'default' && 'bg-primary text-primary-foreground hover:opacity-90',
        variant === 'ghost' && 'hover:bg-secondary',
        variant === 'outline' && 'border border-input bg-transparent hover:bg-secondary',
        className,
      )}
      {...props}
    >
      {children}
    </button>
  )
}

export function ChatInput({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={cn(
        'h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        className,
      )}
      {...props}
    />
  )
}

export function ChatRoot({ theme, children, className }: { theme: string; children: ReactNode; className?: string }) {
  return (
    <div data-chat-theme={theme} className={cn('xvpn-chat-root', className)}>
      {children}
    </div>
  )
}

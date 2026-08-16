import { cn } from '@chat/lib/utils'
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode } from 'react'

export function ChatButton({
  children,
  className,
  variant = 'default',
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: 'default' | 'ghost' | 'outline' | 'safe' }) {
  return (
    <button
      type="button"
      className={cn(
        'inline-flex h-10 items-center justify-center gap-1.5 rounded-[14px] px-4 text-sm font-medium transition-colors disabled:opacity-50',
        variant === 'default' && 'bg-primary text-primary-foreground hover:opacity-90',
        variant === 'ghost' && 'hover:bg-white/8',
        variant === 'outline' && 'border border-white/10 bg-transparent hover:bg-white/8',
        variant === 'safe' &&
          'bg-[var(--safe)] text-[var(--safe-foreground)] shadow-[0_0_18px_-4px_var(--glow-safe)] hover:opacity-90',
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
        'h-10 w-full rounded-[14px] border-0 bg-foreground/[0.06] px-3.5 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
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

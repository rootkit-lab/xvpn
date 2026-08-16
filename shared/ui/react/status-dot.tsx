import { cn } from './cn'

export function StatusDot({ className = '', safe = true }: { className?: string; safe?: boolean }) {
  return (
    <span
      className={cn(safe ? 'status-safe-dot' : 'inline-block rounded-full bg-muted-foreground', 'size-2', className)}
      aria-hidden="true"
    />
  )
}

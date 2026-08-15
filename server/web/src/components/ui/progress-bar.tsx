import { cn } from '@/lib/utils'

/** Barra de progresso indeterminada para ações longas (provisionar Samba/SFTP). */
export function ProgressBar({ className, label }: { className?: string; label?: string }) {
  return (
    <div className={cn('flex flex-col gap-1.5', className)} role="progressbar" aria-busy="true" aria-label={label ?? 'Em progresso'}>
      {label && <p className="text-xs text-muted-foreground">{label}</p>}
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-secondary">
        <div className="h-full w-1/3 animate-[progress-slide_1.1s_ease-in-out_infinite] rounded-full bg-primary" />
      </div>
      <style>{`
        @keyframes progress-slide {
          0% { transform: translateX(-100%); }
          100% { transform: translateX(300%); }
        }
      `}</style>
    </div>
  )
}

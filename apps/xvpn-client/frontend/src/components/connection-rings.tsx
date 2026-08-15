/** Anéis concêntricos estilo Activity Rings do watchOS — decorativo. */
export function ConnectionRings({
  className = '',
  active = false,
  reconnecting = false,
}: {
  className?: string
  active?: boolean
  reconnecting?: boolean
}) {
  const track = 'color-mix(in oklch, var(--foreground) 12%, transparent)'
  const accent = reconnecting ? 'var(--glow-amber)' : 'var(--glow)'
  const spinning = active || reconnecting

  return (
    <div className={`pointer-events-none select-none ${className}`} aria-hidden="true">
      <svg viewBox="0 0 200 200" className="h-full w-full overflow-visible" shapeRendering="geometricPrecision">
        <defs>
          <linearGradient id="watch-ring-grad" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stopColor={accent} stopOpacity="1" />
            <stop offset="100%" stopColor={accent} stopOpacity="0.5" />
          </linearGradient>
        </defs>

        <circle cx="100" cy="100" r="88" fill="none" stroke={track} strokeWidth="10" strokeLinecap="round" />
        <circle cx="100" cy="100" r="72" fill="none" stroke={track} strokeWidth="10" strokeLinecap="round" />
        <circle cx="100" cy="100" r="56" fill="none" stroke={track} strokeWidth="10" strokeLinecap="round" />

        <g
          style={{
            transformOrigin: '100px 100px',
            animation: spinning ? 'watch-ring-spin 16s linear infinite' : undefined,
          }}
        >
          <circle
            cx="100"
            cy="100"
            r="88"
            fill="none"
            stroke="url(#watch-ring-grad)"
            strokeWidth="10"
            strokeLinecap="round"
            strokeDasharray={active ? '400 554' : reconnecting ? '110 554' : '40 554'}
            strokeDashoffset="90"
            className="transition-[stroke-dasharray] duration-700 ease-out"
            opacity={spinning ? 1 : 0.4}
          />
        </g>
        <g
          style={{
            transformOrigin: '100px 100px',
            animation: spinning ? 'watch-ring-spin-rev 22s linear infinite' : undefined,
          }}
        >
          <circle
            cx="100"
            cy="100"
            r="72"
            fill="none"
            stroke={accent}
            strokeWidth="10"
            strokeLinecap="round"
            strokeDasharray={active ? '280 452' : reconnecting ? '90 452' : '30 452'}
            strokeDashoffset="50"
            className="transition-[stroke-dasharray] duration-700 ease-out"
            opacity={spinning ? 0.85 : 0.28}
          />
        </g>
        <g
          style={{
            transformOrigin: '100px 100px',
            animation: spinning ? 'watch-ring-spin 28s linear infinite' : undefined,
          }}
        >
          <circle
            cx="100"
            cy="100"
            r="56"
            fill="none"
            stroke={accent}
            strokeWidth="10"
            strokeLinecap="round"
            strokeDasharray={active ? '190 352' : reconnecting ? '70 352' : '24 352'}
            className="transition-[stroke-dasharray] duration-700 ease-out"
            opacity={spinning ? 0.7 : 0.22}
          />
        </g>
      </svg>
      <style>{`
        @keyframes watch-ring-spin {
          to { transform: rotate(360deg); }
        }
        @keyframes watch-ring-spin-rev {
          to { transform: rotate(-360deg); }
        }
      `}</style>
    </div>
  )
}

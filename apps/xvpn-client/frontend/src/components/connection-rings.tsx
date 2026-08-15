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
  // Ativo = verde "protegido"; reconectando = âmbar; idle = azul marca.
  const accent = reconnecting ? 'var(--glow-amber)' : active ? 'var(--glow-safe)' : 'var(--glow)'
  const accentSoft = reconnecting
    ? 'color-mix(in oklch, var(--glow-amber) 55%, white)'
    : active
      ? 'color-mix(in oklch, var(--glow-safe) 60%, white)'
      : 'color-mix(in oklch, var(--glow) 55%, white)'
  const spinning = active || reconnecting
  const uid = 'xvpn-rings'

  return (
    <div className={`pointer-events-none select-none ${className}`} aria-hidden="true">
      <svg viewBox="0 0 200 200" className="h-full w-full overflow-visible" shapeRendering="geometricPrecision">
        <defs>
          <linearGradient id={`${uid}-grad`} x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stopColor={accentSoft} stopOpacity="1" />
            <stop offset="55%" stopColor={accent} stopOpacity="1" />
            <stop offset="100%" stopColor={accent} stopOpacity="0.35" />
          </linearGradient>
          <filter id={`${uid}-glow`} x="-40%" y="-40%" width="180%" height="180%">
            <feGaussianBlur stdDeviation={active ? 3.2 : 1.4} result="blur" />
            <feMerge>
              <feMergeNode in="blur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>

        <circle cx="100" cy="100" r="88" fill="none" stroke={track} strokeWidth="10" strokeLinecap="round" />
        <circle cx="100" cy="100" r="72" fill="none" stroke={track} strokeWidth="10" strokeLinecap="round" />
        <circle cx="100" cy="100" r="56" fill="none" stroke={track} strokeWidth="10" strokeLinecap="round" />

        <g
          filter={spinning ? `url(#${uid}-glow)` : undefined}
          style={{
            transformOrigin: '100px 100px',
            animation: spinning ? `watch-ring-spin ${active ? '12s' : '16s'} linear infinite` : undefined,
          }}
        >
          <circle
            cx="100"
            cy="100"
            r="88"
            fill="none"
            stroke={`url(#${uid}-grad)`}
            strokeWidth="10"
            strokeLinecap="round"
            strokeDasharray={active ? '420 554' : reconnecting ? '110 554' : '40 554'}
            strokeDashoffset="90"
            className="transition-[stroke-dasharray,opacity] duration-700 ease-out"
            opacity={spinning ? 1 : 0.4}
          />
        </g>
        <g
          filter={spinning ? `url(#${uid}-glow)` : undefined}
          style={{
            transformOrigin: '100px 100px',
            animation: spinning ? `watch-ring-spin-rev ${active ? '18s' : '22s'} linear infinite` : undefined,
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
            strokeDasharray={active ? '300 452' : reconnecting ? '90 452' : '30 452'}
            strokeDashoffset="50"
            className="transition-[stroke-dasharray,opacity] duration-700 ease-out"
            opacity={spinning ? 0.9 : 0.28}
          />
        </g>
        <g
          style={{
            transformOrigin: '100px 100px',
            animation: spinning ? `watch-ring-spin ${active ? '24s' : '28s'} linear infinite` : undefined,
          }}
        >
          <circle
            cx="100"
            cy="100"
            r="56"
            fill="none"
            stroke={accentSoft}
            strokeWidth="10"
            strokeLinecap="round"
            strokeDasharray={active ? '210 352' : reconnecting ? '70 352' : '24 352'}
            className="transition-[stroke-dasharray,opacity] duration-700 ease-out"
            opacity={spinning ? 0.75 : 0.22}
          />
        </g>

        {active && (
          <circle
            cx="100"
            cy="100"
            r="44"
            fill="none"
            stroke={accent}
            strokeWidth="1.5"
            opacity="0.35"
            style={{
              transformOrigin: '100px 100px',
              animation: 'watch-ring-pulse 2.4s ease-in-out infinite',
            }}
          />
        )}
      </svg>
      <style>{`
        @keyframes watch-ring-spin {
          to { transform: rotate(360deg); }
        }
        @keyframes watch-ring-spin-rev {
          to { transform: rotate(-360deg); }
        }
        @keyframes watch-ring-pulse {
          0%, 100% { opacity: 0.2; transform: scale(0.96); }
          50% { opacity: 0.45; transform: scale(1.04); }
        }
      `}</style>
    </div>
  )
}

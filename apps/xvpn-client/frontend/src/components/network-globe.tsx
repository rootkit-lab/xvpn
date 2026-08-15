// Fundo decorativo animado (globo wireframe + arcos de conexão pulsando)
// atrás do botão de conectar — SVG + CSS, sem lib 3D. Puramente
// decorativo: aria-hidden e pointer-events-none.
const NODES = [
  { x: 100, y: 58 },
  { x: 225, y: 78 },
  { x: 55, y: 155 },
  { x: 268, y: 188 },
  { x: 148, y: 228 },
  { x: 318, y: 118 },
  { x: 188, y: 38 },
  { x: 300, y: 210 },
]

const ARCS: [number, number][] = [
  [0, 1],
  [1, 5],
  [2, 4],
  [4, 3],
  [0, 2],
  [3, 5],
  [6, 1],
  [5, 7],
]

function arcPath(a: { x: number; y: number }, b: { x: number; y: number }) {
  const mx = (a.x + b.x) / 2
  const my = (a.y + b.y) / 2 - 36
  return `M ${a.x} ${a.y} Q ${mx} ${my} ${b.x} ${b.y}`
}

export function NetworkGlobe({ className = '' }: { className?: string }) {
  return (
    <div className={`pointer-events-none select-none ${className}`} aria-hidden="true">
      {/* Halo suave atrás do globo — blur CSS evita aliasing do SVG. */}
      <div className="pointer-events-none absolute left-1/2 top-1/2 h-44 w-44 -translate-x-1/2 -translate-y-1/2 rounded-full bg-[radial-gradient(circle,color-mix(in_oklch,var(--glow)_55%,transparent)_0%,transparent_68%)] opacity-50 blur-2xl" />

      <svg
        viewBox="0 0 380 260"
        className="pointer-events-none h-full w-full overflow-visible text-primary"
        aria-hidden="true"
        shapeRendering="geometricPrecision"
      >
        <defs>
          <linearGradient id="globe-ring" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stopColor="var(--color-glow)" stopOpacity="0.05" />
            <stop offset="45%" stopColor="var(--color-glow)" stopOpacity="0.45" />
            <stop offset="100%" stopColor="var(--color-glow)" stopOpacity="0.08" />
          </linearGradient>
          <filter id="soft-glow" x="-40%" y="-40%" width="180%" height="180%">
            <feGaussianBlur stdDeviation="1.4" result="b" />
            <feMerge>
              <feMergeNode in="b" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>

        <g className="animate-drift" style={{ transformOrigin: '190px 130px' }} filter="url(#soft-glow)">
          <circle cx="190" cy="130" r="98" fill="none" stroke="url(#globe-ring)" strokeWidth="1.25" />
          <circle cx="190" cy="130" r="95" fill="none" stroke="currentColor" strokeOpacity="0.12" strokeWidth="1" />
          <ellipse cx="190" cy="130" rx="95" ry="30" fill="none" stroke="currentColor" strokeOpacity="0.16" strokeWidth="1" />
          <ellipse cx="190" cy="130" rx="95" ry="58" fill="none" stroke="currentColor" strokeOpacity="0.12" strokeWidth="1" />
          <ellipse cx="190" cy="130" rx="30" ry="95" fill="none" stroke="currentColor" strokeOpacity="0.12" strokeWidth="1" />
          <ellipse cx="190" cy="130" rx="58" ry="95" fill="none" stroke="currentColor" strokeOpacity="0.1" strokeWidth="1" />

          {ARCS.map(([from, to], i) => (
            <path
              key={i}
              d={arcPath(NODES[from], NODES[to])}
              fill="none"
              stroke="var(--color-glow)"
              strokeWidth="1.35"
              strokeLinecap="round"
              strokeDasharray="5 12"
              opacity="0.5"
              style={{ animation: `dash-flow 4s linear infinite`, animationDelay: `${i * 0.35}s` }}
            />
          ))}

          {NODES.map((n, i) => (
            <g key={i}>
              <circle
                cx={n.x}
                cy={n.y}
                r="8"
                fill="var(--color-glow)"
                opacity="0.2"
                style={{ animation: `node-pulse 2.8s ease-in-out infinite`, animationDelay: `${i * 0.28}s` }}
              />
              <circle cx={n.x} cy={n.y} r="2.25" fill="var(--color-glow)" />
            </g>
          ))}
        </g>
      </svg>
      <style>{`
        @keyframes dash-flow {
          to { stroke-dashoffset: -170; }
        }
        @keyframes node-pulse {
          0%, 100% { opacity: 0.22; transform: scale(1); }
          50% { opacity: 0; transform: scale(1.85); }
        }
      `}</style>
    </div>
  )
}

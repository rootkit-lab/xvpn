// Fundo decorativo animado (globo wireframe + arcos de conexão pulsando)
// atrás do botão de conectar — tudo SVG + CSS, sem lib de 3D/mapa (a
// janela do cliente é pequena, todo KB de bundle importa). Puramente
// decorativo: `aria-hidden` e `pointer-events-none`.
const NODES = [
  { x: 100, y: 60 },
  { x: 220, y: 90 },
  { x: 60, y: 160 },
  { x: 260, y: 190 },
  { x: 150, y: 220 },
  { x: 320, y: 120 },
  { x: 190, y: 40 },
]

const ARCS: [number, number][] = [
  [0, 1],
  [1, 5],
  [2, 4],
  [4, 3],
  [0, 2],
  [3, 5],
]

function arcPath(a: { x: number; y: number }, b: { x: number; y: number }) {
  const mx = (a.x + b.x) / 2
  const my = (a.y + b.y) / 2 - 40
  return `M ${a.x} ${a.y} Q ${mx} ${my} ${b.x} ${b.y}`
}

export function NetworkGlobe({ className = '' }: { className?: string }) {
  return (
    <div className={`pointer-events-none select-none ${className}`} aria-hidden="true">
      <svg viewBox="0 0 380 260" className="h-full w-full overflow-visible text-primary">
        <g className="animate-drift" style={{ transformOrigin: '190px 130px' }}>
          <circle cx="190" cy="130" r="95" fill="none" stroke="currentColor" strokeOpacity="0.18" strokeWidth="1" />
          <ellipse cx="190" cy="130" rx="95" ry="32" fill="none" stroke="currentColor" strokeOpacity="0.14" strokeWidth="1" />
          <ellipse cx="190" cy="130" rx="95" ry="60" fill="none" stroke="currentColor" strokeOpacity="0.14" strokeWidth="1" />
          <ellipse cx="190" cy="130" rx="32" ry="95" fill="none" stroke="currentColor" strokeOpacity="0.14" strokeWidth="1" />
          <ellipse cx="190" cy="130" rx="60" ry="95" fill="none" stroke="currentColor" strokeOpacity="0.14" strokeWidth="1" />

          {ARCS.map(([from, to], i) => (
            <path
              key={i}
              d={arcPath(NODES[from], NODES[to])}
              fill="none"
              stroke="var(--color-glow)"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeDasharray="6 10"
              opacity="0.55"
              style={{ animation: `dash-flow 3.5s linear infinite`, animationDelay: `${i * 0.4}s` }}
            />
          ))}

          {NODES.map((n, i) => (
            <g key={i}>
              <circle
                cx={n.x}
                cy={n.y}
                r="7"
                fill="var(--color-glow)"
                opacity="0.25"
                style={{ animation: `node-pulse 2.8s ease-in-out infinite`, animationDelay: `${i * 0.3}s` }}
              />
              <circle cx={n.x} cy={n.y} r="2.5" fill="var(--color-glow)" />
            </g>
          ))}
        </g>
      </svg>
      <style>{`
        @keyframes dash-flow {
          to { stroke-dashoffset: -160; }
        }
        @keyframes node-pulse {
          0%, 100% { r: 5; opacity: 0.25; }
          50% { r: 12; opacity: 0; }
        }
      `}</style>
    </div>
  )
}

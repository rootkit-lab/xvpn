import type { MeshServer } from '@/lib/api'

function fqdn(server: MeshServer) {
  if (server.role === 'external') {
    return server.hostname
  }
  return `${server.hostname}.corp.ihuull.com`
}

export function ServerConsole({ server }: { server: MeshServer }) {
  const host = fqdn(server)
  const prompt = `root@${server.hostname}:~#`
  const lines = [
    `${prompt} info`,
    `host     ${host}`,
    `name     ${server.name}`,
    `ipv4     ${server.ipv4 || '—'}`,
    `wg0      ${server.wg_ip || '—'}`,
    `role     ${server.role}`,
    `status   ${server.status}`,
    `region   ${server.region || '—'}`,
    `size     ${server.size || '—'}`,
    `policy   ${server.protected ? 'inventory-only (sem enroll/destroy)' : 'mesh'}`,
  ]
  if (server.notes?.trim()) {
    lines.push(`notes    ${server.notes.trim().replace(/\s+/g, ' ')}`)
  }
  lines.push(`${prompt} `)

  return (
    <div className="watch-complication overflow-hidden rounded-[18px]">
      <div className="flex items-center gap-2 border-b border-white/10 px-3 py-2">
        <span className="size-2.5 rounded-full bg-destructive/80" />
        <span className="size-2.5 rounded-full bg-amber-400/80" />
        <span className="size-2.5 rounded-full bg-[color:var(--safe)]/80" />
        <span className="hud-mono ml-2 text-[11px] text-muted-foreground">xterm — {host}</span>
      </div>
      <pre className="scanline hud-mono relative m-0 max-h-72 overflow-auto px-4 py-3 text-[12px] leading-6 text-[color:var(--safe)]">
        {lines.join('\n')}
      </pre>
    </div>
  )
}

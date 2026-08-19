/** Quantos nós cabem no painel sem virar sopa de linhas. */
export function plexusParticleCount(width: number, height: number): number {
  const area = Math.max(0, width) * Math.max(0, height)
  return Math.min(68, Math.max(22, Math.round(area / 14000)))
}

export function plexusLinkDistance(width: number, height: number): number {
  return Math.min(168, Math.max(96, Math.round(Math.min(width, height) * 0.28)))
}

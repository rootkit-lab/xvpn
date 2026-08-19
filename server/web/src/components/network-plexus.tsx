import { useEffect, useRef } from 'react'
import { plexusLinkDistance, plexusParticleCount } from '@/lib/network-plexus'

type Particle = { x: number; y: number; vx: number; vy: number }

/** Plexus de pontos ligados — canvas 2D, tokens `--primary` / `--glow`. */
export function NetworkPlexus({ className = '' }: { className?: string }) {
  const ref = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const surface = ref.current
    const draw = surface?.getContext('2d', { alpha: true })
    if (!surface || !draw) return

    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    const particles: Particle[] = []
    const colors = { dot: '', glow: '' }
    let width = 0
    let height = 0
    let link = 120
    let raf = 0
    let running = true

    const readColors = () => {
      surface.style.color = 'var(--primary)'
      colors.dot = getComputedStyle(surface).color
      surface.style.color = 'var(--glow)'
      colors.glow = getComputedStyle(surface).color
      surface.style.color = ''
    }

    const seed = () => {
      const next = plexusParticleCount(width, height)
      link = plexusLinkDistance(width, height)
      particles.length = 0
      for (let i = 0; i < next; i++) {
        const angle = Math.random() * Math.PI * 2
        const speed = 0.12 + Math.random() * 0.22
        particles.push({
          x: Math.random() * width,
          y: Math.random() * height,
          vx: Math.cos(angle) * speed,
          vy: Math.sin(angle) * speed,
        })
      }
    }

    const resize = () => {
      const parent = surface.parentElement
      if (!parent) return
      const dpr = Math.min(window.devicePixelRatio || 1, 2)
      width = parent.clientWidth
      height = parent.clientHeight
      surface.width = Math.max(1, Math.floor(width * dpr))
      surface.height = Math.max(1, Math.floor(height * dpr))
      surface.style.width = `${width}px`
      surface.style.height = `${height}px`
      draw.setTransform(dpr, 0, 0, dpr, 0, 0)
      readColors()
      seed()
    }

    const step = () => {
      if (!running) return
      draw.clearRect(0, 0, width, height)

      for (const p of particles) {
        if (!reduceMotion) {
          p.x += p.vx
          p.y += p.vy
          if (p.x < 0 || p.x > width) p.vx *= -1
          if (p.y < 0 || p.y > height) p.vy *= -1
          p.x = Math.min(width, Math.max(0, p.x))
          p.y = Math.min(height, Math.max(0, p.y))
        }
      }

      for (let i = 0; i < particles.length; i++) {
        const a = particles[i]
        for (let j = i + 1; j < particles.length; j++) {
          const b = particles[j]
          const dx = a.x - b.x
          const dy = a.y - b.y
          const dist = Math.hypot(dx, dy)
          if (dist > link) continue
          draw.strokeStyle = colors.glow
          draw.globalAlpha = (1 - dist / link) * 0.42
          draw.lineWidth = 1
          draw.beginPath()
          draw.moveTo(a.x, a.y)
          draw.lineTo(b.x, b.y)
          draw.stroke()
        }
      }

      draw.globalAlpha = 1
      for (const p of particles) {
        draw.fillStyle = colors.glow
        draw.globalAlpha = 0.18
        draw.beginPath()
        draw.arc(p.x, p.y, 5.5, 0, Math.PI * 2)
        draw.fill()
        draw.fillStyle = colors.dot
        draw.globalAlpha = 0.95
        draw.beginPath()
        draw.arc(p.x, p.y, 1.7, 0, Math.PI * 2)
        draw.fill()
      }
      draw.globalAlpha = 1

      if (!reduceMotion) raf = requestAnimationFrame(step)
    }

    const ro = new ResizeObserver(resize)
    if (surface.parentElement) ro.observe(surface.parentElement)
    resize()
    step()

    const onVis = () => {
      if (document.hidden) {
        running = false
        cancelAnimationFrame(raf)
        return
      }
      if (!running) {
        running = true
        if (!reduceMotion) raf = requestAnimationFrame(step)
      }
    }
    document.addEventListener('visibilitychange', onVis)

    return () => {
      running = false
      cancelAnimationFrame(raf)
      ro.disconnect()
      document.removeEventListener('visibilitychange', onVis)
    }
  }, [])

  return <canvas ref={ref} className={`pointer-events-none absolute inset-0 ${className}`} aria-hidden="true" />
}

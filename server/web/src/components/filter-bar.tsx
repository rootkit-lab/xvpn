import { type ReactNode, useEffect, useState } from 'react'
import { Input } from '@/components/ui/input'

export function FilterBar({
  q,
  onQChange,
  placeholder = 'Buscar…',
  children,
}: {
  q: string
  onQChange: (q: string) => void
  placeholder?: string
  children?: ReactNode
}) {
  const [local, setLocal] = useState(q)

  useEffect(() => {
    setLocal(q)
  }, [q])

  useEffect(() => {
    const id = window.setTimeout(() => {
      if (local !== q) onQChange(local)
    }, 250)
    return () => window.clearTimeout(id)
  }, [local, onQChange, q])

  return (
    <div className="flex flex-wrap items-center gap-3">
      <Input
        value={local}
        onChange={(e) => setLocal(e.target.value)}
        placeholder={placeholder}
        className="max-w-xs"
      />
      {children}
    </div>
  )
}

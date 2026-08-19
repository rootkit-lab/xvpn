import { useMemo, useState } from 'react'
import type { User } from '@/lib/api'
import { ROLE_BADGE_VARIANT, ROLE_LABELS } from '@/lib/roles'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { FilterBar } from '@/components/filter-bar'
import { EmptyState } from '@/components/pagination'

export function UserPicker({
  users,
  selected,
  onToggle,
}: {
  users: User[]
  selected: Set<number>
  onToggle: (id: number) => void
}) {
  const [q, setQ] = useState('')
  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase()
    if (!needle) return users
    return users.filter((u) => u.username.toLowerCase().includes(needle))
  }, [users, q])

  return (
    <div className="flex flex-col gap-3">
      <FilterBar q={q} onQChange={setQ} placeholder="Filtrar usuários" />
      <div className="flex max-h-72 flex-col gap-1 overflow-y-auto">
        {filtered.length === 0 ? (
          <EmptyState title="Nenhum usuário neste filtro." />
        ) : (
          filtered.map((u) => (
            <div key={u.id} className="flex items-center gap-3 rounded-md px-2 py-1.5 hover:bg-accent">
              <Checkbox
                id={`picker-user-${u.id}`}
                checked={selected.has(u.id)}
                onCheckedChange={() => onToggle(u.id)}
              />
              <Label htmlFor={`picker-user-${u.id}`} className="flex-1 cursor-pointer font-normal">
                {u.username}
              </Label>
              <Badge variant={ROLE_BADGE_VARIANT[u.role]}>{ROLE_LABELS[u.role]}</Badge>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

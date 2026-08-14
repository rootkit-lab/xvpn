import { assignableRoles, ROLE_LABELS, type Role } from '@/lib/roles'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

// RoleSelect só oferece papéis que `caller` pode conceder (assignableRoles
// espelha store.Role.CanManage) — usado tanto ao criar/editar usuário
// quanto ao provisionar a partir da waitlist, para nunca desenhar uma
// opção que o backend vai rejeitar com 403.
export function RoleSelect({
  value,
  onChange,
  caller,
}: {
  value: Role
  onChange: (role: Role) => void
  caller: Role | undefined
}) {
  const options = assignableRoles(caller)
  return (
    <Select value={value} onValueChange={(v) => onChange(v as Role)}>
      <SelectTrigger className="w-full">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {options.map((r) => (
          <SelectItem key={r} value={r}>
            {ROLE_LABELS[r]}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

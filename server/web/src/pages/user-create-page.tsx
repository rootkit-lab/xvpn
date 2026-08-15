import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { api, ApiError } from '@/lib/api'
import { useAuth } from '@/lib/auth-context'
import { type Role } from '@/lib/roles'
import { RoleSelect } from '@/components/role-select'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

export function UserCreatePage() {
  const { user: caller } = useAuth()
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<Role>('member')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      const created = await api.createUser(username, password, role)
      toast.success(`Usuário "${created.username}" criado`)
      navigate(`/admin/users/${created.id}`)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Falha ao criar usuário')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Card className="max-w-lg">
      <CardHeader>
        <CardTitle>Novo usuário</CardTitle>
        <CardDescription>
          Cria uma conta com acesso à VPN. Depois gere um convite na ficha para registrar o primeiro dispositivo.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="new-username">Usuário</Label>
            <Input id="new-username" required value={username} onChange={(e) => setUsername(e.target.value)} />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="new-password">Senha</Label>
            <Input
              id="new-password"
              type="password"
              required
              minLength={8}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label>Papel</Label>
            <RoleSelect value={role} onChange={setRole} caller={caller?.role} />
          </div>
          {error && <p className="text-sm text-destructive">{error}</p>}
          <div className="flex gap-2">
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Criando…' : 'Criar usuário'}
            </Button>
            <Button type="button" variant="outline" onClick={() => navigate('/admin/users')}>
              Cancelar
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}

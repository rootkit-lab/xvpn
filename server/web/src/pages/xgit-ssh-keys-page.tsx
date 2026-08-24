import { useCallback, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { KeyRound, Trash2 } from 'lucide-react'
import { api, ApiError, type ForgeSSHKey } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { ProgressBar } from '@/components/ui/progress-bar'

function formatWhen(iso?: string | null) {
  if (!iso) return 'Never'
  try {
    return new Intl.DateTimeFormat('pt-BR', { dateStyle: 'medium' }).format(new Date(iso))
  } catch {
    return iso
  }
}

export function XgitSSHKeysPage() {
  const fetchKeys = useCallback(() => api.listMyForgeSSHKeys(), [])
  const { data, loading, reload } = usePollingData(fetchKeys, 30_000)
  const keys = data?.keys ?? []

  const [title, setTitle] = useState('')
  const [publicKey, setPublicKey] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [deletingId, setDeletingId] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)

  async function handleAdd(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await api.createMyForgeSSHKey({ title: title.trim(), public_key: publicKey.trim() })
      setTitle('')
      setPublicKey('')
      toast.success('SSH key added')
      await reload()
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : 'Failed to add SSH key'
      setError(msg)
      toast.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelete(key: ForgeSSHKey) {
    setDeletingId(key.id)
    try {
      await api.deleteMyForgeSSHKey(key.id)
      toast.success('SSH key removed')
      await reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Failed to remove SSH key')
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <div className="flex flex-col gap-8">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">SSH keys</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Authentication keys for <code className="text-xs">git@xgit.corp.ihuull.com</code>. Devices with XVPN
          connected register automatically.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Authentication keys</CardTitle>
          <CardDescription>SSH keys linked to your account.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {loading ? (
            <Skeleton className="h-24 w-full" />
          ) : keys.length === 0 ? (
            <p className="text-sm text-muted-foreground">No SSH keys yet. Connect XVPN or add one below.</p>
          ) : (
            keys.map((key) => (
              <div
                key={key.id}
                className="flex flex-col gap-2 rounded-lg border border-border/60 p-4 sm:flex-row sm:items-start sm:justify-between"
              >
                <div className="flex gap-3">
                  <KeyRound className="mt-0.5 size-5 shrink-0 text-emerald-500" />
                  <div>
                    <p className="font-medium">{key.title}</p>
                    <p className="font-mono text-xs text-muted-foreground">{key.fingerprint}</p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      SSH · Added {formatWhen(key.created_at)}
                      {key.device_id ? ' · from XVPN device' : ''}
                      {key.last_used_at ? ` · Last used ${formatWhen(key.last_used_at)}` : ''}
                    </p>
                  </div>
                </div>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={deletingId === key.id}
                  onClick={() => void handleDelete(key)}
                >
                  <Trash2 className="size-4" />
                  Delete
                </Button>
              </div>
            ))
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Add new SSH key</CardTitle>
          <CardDescription>Paste a public key (ssh-ed25519, ssh-rsa, …).</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleAdd} className="flex max-w-xl flex-col gap-3">
            <div className="flex flex-col gap-2">
              <Label htmlFor="ssh-key-title">Title</Label>
              <Input
                id="ssh-key-title"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="pop-os"
                disabled={submitting}
                required
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="ssh-key-body">Key</Label>
              <Textarea
                id="ssh-key-body"
                className="min-h-28 font-mono text-sm"
                value={publicKey}
                onChange={(e) => setPublicKey(e.target.value)}
                placeholder="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5..."
                spellCheck={false}
                disabled={submitting}
                required
              />
            </div>
            {submitting && <ProgressBar label="Adding key…" />}
            {error && <p className="text-sm text-destructive">{error}</p>}
            <div>
              <Button type="submit" disabled={submitting}>
                {submitting ? 'Adding…' : 'Add SSH key'}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">GPG keys</CardTitle>
          <CardDescription>
            Signed commits and tags. Upload of GPG keys is planned — not available yet.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <p className="text-sm text-muted-foreground">
            There are no GPG keys associated with your account.
          </p>
          <Button type="button" variant="outline" disabled title="Coming soon">
            New GPG key
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}

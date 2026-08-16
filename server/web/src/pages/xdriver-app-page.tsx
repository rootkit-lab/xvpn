import { useCallback, useRef, useState } from 'react'
import { toast } from 'sonner'
import {
  ChevronRight,
  Download,
  File,
  Folder,
  FolderPlus,
  HardDrive,
  Trash2,
  Upload,
  Users,
} from 'lucide-react'
import { api, ApiError, type DriverEntry, type DriverRoot } from '@/lib/api'
import { usePollingData } from '@/hooks/use-polling-data'
import { useAuth } from '@/lib/auth-context'
import { formatBytes } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { StoreShell } from '@/components/layout/store-shell'

export function XDriverLayout() {
  return <StoreShell kind="xdriver" />
}

const ROOTS: { id: DriverRoot; label: string; hint: string; icon: typeof HardDrive }[] = [
  { id: 'home', label: 'Meu Drive', hint: 'Pasta pessoal', icon: HardDrive },
  { id: 'shared', label: 'Compartilhado', hint: 'Todos na VPN', icon: Users },
]

export function XDriverAppPage() {
  const { user } = useAuth()
  const [root, setRoot] = useState<DriverRoot>('shared')
  const [path, setPath] = useState('')
  const [selected, setSelected] = useState<string | null>(null)
  const [folderName, setFolderName] = useState('')
  const [busy, setBusy] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  const fetchList = useCallback(() => api.listDriver(root, path), [root, path])
  const { data, loading, error, reload } = usePollingData(fetchList, 12_000)

  const crumbs = path ? path.split('/').filter(Boolean) : []
  const homeOff = root === 'home' && !user?.samba_enabled && !user?.sftp_enabled

  function openRoot(next: DriverRoot) {
    setRoot(next)
    setPath('')
    setSelected(null)
  }

  function openDir(rel: string) {
    setPath(rel)
    setSelected(null)
  }

  async function makeFolder() {
    const name = folderName.trim()
    if (!name) return
    setBusy(true)
    try {
      await api.mkdirDriver(root, path, name)
      setFolderName('')
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao criar pasta')
    } finally {
      setBusy(false)
    }
  }

  async function onUpload(files: FileList | null) {
    if (!files?.length) return
    setBusy(true)
    try {
      for (const file of files) {
        await api.uploadDriver(root, path, file)
      }
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha no envio')
    } finally {
      setBusy(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  async function downloadSelected() {
    const item = data?.items.find((e) => e.path === selected)
    if (!item || item.is_dir) return
    try {
      await api.downloadDriver(root, item.path, item.name)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha no download')
    }
  }

  async function removeSelected() {
    const item = data?.items.find((e) => e.path === selected)
    if (!item) return
    if (!window.confirm(`Apagar ${item.name}?`)) return
    setBusy(true)
    try {
      await api.rmDriver(root, item.path)
      setSelected(null)
      reload()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao apagar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="mx-auto flex h-full w-full max-w-6xl gap-4 px-4 py-5 md:px-6">
      <aside className="hidden w-52 shrink-0 flex-col gap-1 md:flex">
        {ROOTS.map(({ id, label, hint, icon: Icon }) => (
          <button
            key={id}
            type="button"
            onClick={() => openRoot(id)}
            className={cn('nav-link w-full text-left', root === id && 'nav-link-active')}
          >
            <Icon className="size-4" />
            <span className="min-w-0">
              <span className="block">{label}</span>
              <span className="block text-[11px] font-normal text-muted-foreground">{hint}</span>
            </span>
          </button>
        ))}
      </aside>

      <div className="flex min-w-0 flex-1 flex-col gap-4">
        <div className="flex flex-wrap items-center gap-2 md:hidden">
          {ROOTS.map(({ id, label }) => (
            <Button key={id} size="sm" className="rounded-full" variant={root === id ? 'default' : 'outline'} onClick={() => openRoot(id)}>
              {label}
            </Button>
          ))}
        </div>

        <div className="flex flex-wrap items-center gap-2 text-sm">
          <button type="button" className="text-primary hover:underline" onClick={() => openDir('')}>
            {root === 'home' ? 'Meu Drive' : 'Compartilhado'}
          </button>
          {crumbs.map((part, i) => {
            const rel = crumbs.slice(0, i + 1).join('/')
            return (
              <span key={rel} className="flex items-center gap-2">
                <ChevronRight className="size-3 text-muted-foreground" />
                <button type="button" className="text-primary hover:underline" onClick={() => openDir(rel)}>
                  {part}
                </button>
              </span>
            )
          })}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Input
            value={folderName}
            onChange={(e) => setFolderName(e.target.value)}
            placeholder="Nova pasta"
            className="max-w-48"
            disabled={busy || homeOff}
          />
          <Button size="sm" className="rounded-full" disabled={busy || homeOff || !folderName.trim()} onClick={makeFolder}>
            <FolderPlus className="size-4" />
            Criar
          </Button>
          <Button size="sm" className="rounded-full" variant="secondary" disabled={busy || homeOff} onClick={() => fileRef.current?.click()}>
            <Upload className="size-4" />
            Enviar
          </Button>
          <input ref={fileRef} type="file" className="hidden" multiple onChange={(e) => onUpload(e.target.files)} />
          <Button size="sm" variant="outline" className="rounded-full" disabled={!selected || data?.items.find((e) => e.path === selected)?.is_dir} onClick={downloadSelected}>
            <Download className="size-4" />
            Baixar
          </Button>
          <Button size="sm" variant="outline" className="rounded-full" disabled={!selected || busy} onClick={removeSelected}>
            <Trash2 className="size-4" />
            Apagar
          </Button>
        </div>

        {homeOff && (
          <p className="text-sm text-muted-foreground">Meu Drive está desligado nesta conta. Peça Samba ou SFTP no painel.</p>
        )}
        {error && <p className="text-sm text-destructive">{error}</p>}

        {loading || !data ? (
          <Skeleton className="h-64 w-full rounded-[22px]" />
        ) : data.items.length === 0 ? (
          <p className="text-sm text-muted-foreground">Pasta vazia. Envie um arquivo ou crie uma pasta.</p>
        ) : (
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {data.items.map((item) => (
              <FileTile
                key={item.path}
                item={item}
                selected={selected === item.path}
                onSelect={() => setSelected(item.path)}
                onOpen={() => item.is_dir && openDir(item.path)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function FileTile({
  item,
  selected,
  onSelect,
  onOpen,
}: {
  item: DriverEntry
  selected: boolean
  onSelect: () => void
  onOpen: () => void
}) {
  const Icon = item.is_dir ? Folder : File
  return (
    <button
      type="button"
      onClick={onSelect}
      onDoubleClick={onOpen}
      className={cn(
        'watch-complication flex flex-col items-start gap-3 rounded-[18px] p-4 text-left transition-colors',
        selected && 'ring-2 ring-primary',
      )}
    >
      <span className="icon-well flex size-10 items-center justify-center rounded-[12px]">
        <Icon className="size-4" />
      </span>
      <span className="w-full truncate font-display text-sm font-semibold">{item.name}</span>
      <span className="text-xs text-muted-foreground">{item.is_dir ? 'Pasta' : formatBytes(item.size)}</span>
    </button>
  )
}

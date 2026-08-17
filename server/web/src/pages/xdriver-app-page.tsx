import { useCallback, useRef, useState, type DragEvent } from 'react'
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
import { useDocumentTitleOverride } from '@/components/layout/document-title'

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
  const [dragOver, setDragOver] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  const fetchList = useCallback(() => api.listDriver(root, path), [root, path])
  const { data, loading, error, reload } = usePollingData(fetchList, 12_000)

  const crumbs = path ? path.split('/').filter(Boolean) : []
  const rootLabel = root === 'home' ? 'Meu Drive' : 'Compartilhado'
  const folder = crumbs[crumbs.length - 1]
  useDocumentTitleOverride(folder ? `${folder} · ${rootLabel}` : rootLabel)
  const homeOff = root === 'home' && !user?.samba_enabled && !user?.sftp_enabled
  const selectedItem = data?.items.find((e) => e.path === selected)
  const canDownload = Boolean(selectedItem && !selectedItem.is_dir)

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
    const item = selectedItem
    if (!item || item.is_dir) return
    try {
      await api.downloadDriver(root, item.path, item.name)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha no download')
    }
  }

  async function removeSelected() {
    const item = selectedItem
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

  function onDrop(e: DragEvent) {
    e.preventDefault()
    setDragOver(false)
    if (homeOff || busy) return
    void onUpload(e.dataTransfer.files)
  }

  return (
    <div className="flex h-full min-h-[calc(100svh-8rem)] w-full min-w-0 gap-5 px-4 py-5 md:px-6">
      <aside className="hidden w-56 shrink-0 flex-col gap-1 md:flex">
        <p className="hud-label px-3 pb-2 text-muted-foreground/70">Locais</p>
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
            <Button
              key={id}
              size="sm"
              className="rounded-full"
              variant={root === id ? 'default' : 'outline'}
              onClick={() => openRoot(id)}
            >
              {label}
            </Button>
          ))}
        </div>

        <nav className="flex flex-wrap items-center gap-1.5 text-sm" aria-label="Caminho">
          <button type="button" className="font-display font-semibold text-primary hover:underline" onClick={() => openDir('')}>
            {root === 'home' ? 'Meu Drive' : 'Compartilhado'}
          </button>
          {crumbs.map((part, i) => {
            const rel = crumbs.slice(0, i + 1).join('/')
            return (
              <span key={rel} className="flex items-center gap-1.5">
                <ChevronRight className="size-3.5 text-muted-foreground" />
                <button type="button" className="text-foreground/90 hover:underline" onClick={() => openDir(rel)}>
                  {part}
                </button>
              </span>
            )
          })}
        </nav>

        <div className="watch-complication flex flex-wrap items-center gap-2 rounded-[18px] p-2.5">
          <Input
            value={folderName}
            onChange={(e) => setFolderName(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && void makeFolder()}
            placeholder="Nova pasta"
            className="max-w-52"
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
          <span className="hidden flex-1 sm:block" />
          <Button size="sm" variant="outline" className="rounded-full" disabled={!canDownload} onClick={downloadSelected}>
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

        <div
          className={cn(
            'min-h-64 flex-1 rounded-[22px] transition-colors',
            dragOver && 'ring-2 ring-primary/60',
          )}
          onDragOver={(e) => {
            e.preventDefault()
            if (!homeOff && !busy) setDragOver(true)
          }}
          onDragLeave={() => setDragOver(false)}
          onDrop={onDrop}
        >
          {loading || !data ? (
            <Skeleton className="h-64 w-full rounded-[22px]" />
          ) : data.items.length === 0 ? (
            <div className="watch-complication flex h-64 flex-col items-center justify-center gap-2 rounded-[22px] px-6 text-center">
              <Folder className="size-8 text-muted-foreground" />
              <p className="font-display text-sm font-semibold">Pasta vazia</p>
              <p className="max-w-sm text-sm text-muted-foreground">
                Arraste arquivos para cá, envie pelo botão ou crie uma pasta.
              </p>
            </div>
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
      onClick={() => (item.is_dir ? onOpen() : onSelect())}
      onDoubleClick={onOpen}
      className={cn(
        'watch-complication flex flex-col items-start gap-3 rounded-[18px] p-4 text-left transition-colors hover:bg-white/6',
        selected && 'ring-2 ring-primary',
      )}
    >
      <span className="icon-well flex size-11 items-center justify-center rounded-[12px]">
        <Icon className="size-5" />
      </span>
      <span className="w-full truncate font-display text-sm font-semibold">{item.name}</span>
      <span className="text-xs text-muted-foreground">{item.is_dir ? 'Pasta' : formatBytes(item.size)}</span>
    </button>
  )
}

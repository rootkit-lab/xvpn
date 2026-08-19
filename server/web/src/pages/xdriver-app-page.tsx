import { useCallback, useEffect, useRef, useState, type DragEvent, type MouseEvent, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import {
  Archive,
  ChevronRight,
  Download,
  Eye,
  File,
  FilePenLine,
  Folder,
  FolderKanban,
  FolderPlus,
  HardDrive,
  Trash2,
  Upload,
  Users,
} from 'lucide-react'
import { api, ApiError, driverFileURL, type DriverEntry, type DriverRoot } from '@/lib/api'
import { archiveExtractable, driverFileKind, driverItemHref, driverOpenMode } from '@/lib/driver-kind'
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

const BASE_ROOTS: { id: DriverRoot; label: string; hint: string; icon: typeof HardDrive }[] = [
  { id: 'home', label: 'Meu Drive', hint: 'Pasta pessoal', icon: HardDrive },
  { id: 'shared', label: 'Compartilhado', hint: 'Todos na VPN', icon: Users },
]

type MenuState = { x: number; y: number; item: DriverEntry }

function rootLabel(root: DriverRoot, projectName?: string): string {
  if (root === 'home') return 'Meu Drive'
  if (root === 'shared') return 'Compartilhado'
  return projectName || root.slice('project:'.length)
}

export function XDriverAppPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [root, setRoot] = useState<DriverRoot>('shared')
  const [path, setPath] = useState('')
  const [selected, setSelected] = useState<string | null>(null)
  const [folderName, setFolderName] = useState('')
  const [busy, setBusy] = useState(false)
  const [dragOver, setDragOver] = useState(false)
  const [menu, setMenu] = useState<MenuState | null>(null)
  const fileRef = useRef<HTMLInputElement>(null)

  const fetchList = useCallback(() => api.listDriver(root, path), [root, path])
  const fetchProjects = useCallback(() => api.listProjects(), [])
  const { data, loading, error, reload } = usePollingData(fetchList, 12_000)
  const { data: projects } = usePollingData(fetchProjects, 30_000)
  const projectRoots = (projects?.items ?? [])
    .filter((p) => p.files_enabled)
    .map((p) => ({
      id: `project:${p.slug}` as DriverRoot,
      label: p.name,
      hint: `projeto ${p.slug}`,
      icon: FolderKanban,
    }))
  const roots = [...BASE_ROOTS, ...projectRoots]

  const crumbs = path ? path.split('/').filter(Boolean) : []
  const currentLabel = rootLabel(
    root,
    projectRoots.find((r) => r.id === root)?.label,
  )
  const folder = crumbs[crumbs.length - 1]
  useDocumentTitleOverride(folder ? `${folder} · ${currentLabel}` : currentLabel)
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
    setMenu(null)
  }

  function openItem(item: DriverEntry) {
    const kind = driverFileKind(item.name, item.is_dir)
    const mode = driverOpenMode(kind)
    if (mode === 'folder') {
      openDir(item.path)
      return
    }
    if (mode === 'edit' || mode === 'view') {
      navigate(driverItemHref(mode, root, item.path))
      return
    }
    if (kind === 'archive' && archiveExtractable(item.name)) {
      void extractItem(item)
      return
    }
    void downloadItem(item)
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

  async function downloadItem(item: DriverEntry) {
    if (item.is_dir) return
    try {
      await api.downloadDriver(root, item.path, item.name)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha no download')
    }
  }

  async function downloadSelected() {
    if (selectedItem) await downloadItem(selectedItem)
  }

  async function extractItem(item: DriverEntry) {
    if (!archiveExtractable(item.name)) {
      toast.error('Só zip e tar.gz. RAR/7z: baixe e extraia no computador.')
      return
    }
    setBusy(true)
    setMenu(null)
    try {
      const out = await api.extractDriver(root, item.path)
      toast.success('Extraído')
      openDir(out.path)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao extrair')
    } finally {
      setBusy(false)
    }
  }

  async function removeItem(item: DriverEntry) {
    if (!window.confirm(`Apagar ${item.name}?`)) return
    setBusy(true)
    setMenu(null)
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

  async function removeSelected() {
    if (selectedItem) await removeItem(selectedItem)
  }

  function onContext(e: MouseEvent, item: DriverEntry) {
    e.preventDefault()
    setSelected(item.path)
    setMenu({ x: e.clientX, y: e.clientY, item })
  }

  function onDrop(e: DragEvent) {
    e.preventDefault()
    setDragOver(false)
    if (homeOff || busy) return
    void onUpload(e.dataTransfer.files)
  }

  useEffect(() => {
    if (!menu) return
    const close = () => setMenu(null)
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') close()
    }
    window.addEventListener('click', close)
    window.addEventListener('keydown', onKey)
    return () => {
      window.removeEventListener('click', close)
      window.removeEventListener('keydown', onKey)
    }
  }, [menu])

  return (
    <div className="flex h-full min-h-[calc(100svh-8rem)] w-full min-w-0 gap-5 px-4 py-5 md:px-6">
      <aside className="hidden w-56 shrink-0 flex-col gap-1 md:flex">
        <p className="hud-label px-3 pb-2 text-muted-foreground/70">Locais</p>
        {roots.map(({ id, label, hint, icon: Icon }) => (
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
          {roots.map(({ id, label }) => (
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
            {currentLabel}
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
                  root={root}
                  item={item}
                  selected={selected === item.path}
                  onSelect={() => setSelected(item.path)}
                  onOpen={() => openItem(item)}
                  onMenu={(e) => onContext(e, item)}
                />
              ))}
            </div>
          )}
        </div>
      </div>

      {menu && (
        <ContextMenu
          x={menu.x}
          y={menu.y}
          item={menu.item}
          busy={busy}
          onOpen={() => openItem(menu.item)}
          onDownload={() => void downloadItem(menu.item)}
          onExtract={() => void extractItem(menu.item)}
          onDelete={() => void removeItem(menu.item)}
        />
      )}
    </div>
  )
}

function FileTile({
  root,
  item,
  selected,
  onSelect,
  onOpen,
  onMenu,
}: {
  root: DriverRoot
  item: DriverEntry
  selected: boolean
  onSelect: () => void
  onOpen: () => void
  onMenu: (e: MouseEvent) => void
}) {
  const kind = driverFileKind(item.name, item.is_dir)
  const thumb = kind === 'image' ? driverFileURL(root, item.path, true) : null
  return (
    <button
      type="button"
      onClick={() => (item.is_dir ? onOpen() : onSelect())}
      onDoubleClick={onOpen}
      onContextMenu={onMenu}
      className={cn(
        'watch-complication flex flex-col items-start gap-3 overflow-hidden rounded-[18px] p-3 text-left transition-colors hover:bg-white/6',
        selected && 'ring-2 ring-primary',
      )}
    >
      <span className="relative flex h-28 w-full items-center justify-center overflow-hidden rounded-[14px] bg-black/25">
        {thumb ? (
          <img src={thumb} alt="" className="h-full w-full object-cover" />
        ) : (
          <KindGlyph kind={kind} />
        )}
      </span>
      <span className="w-full truncate font-display text-sm font-semibold">{item.name}</span>
      <span className="text-xs text-muted-foreground">
        {kind === 'folder' ? 'Pasta' : kind === 'archive' ? `Compactado · ${formatBytes(item.size)}` : formatBytes(item.size)}
      </span>
    </button>
  )
}

function KindGlyph({ kind }: { kind: ReturnType<typeof driverFileKind> }) {
  const Icon = kind === 'folder' ? Folder : kind === 'archive' ? Archive : kind === 'text' ? FilePenLine : kind === 'video' || kind === 'audio' || kind === 'pdf' ? Eye : File
  return (
    <span className="icon-well flex size-12 items-center justify-center rounded-[14px]">
      <Icon className="size-5" />
    </span>
  )
}

function ContextMenu({
  x,
  y,
  item,
  busy,
  onOpen,
  onDownload,
  onExtract,
  onDelete,
}: {
  x: number
  y: number
  item: DriverEntry
  busy: boolean
  onOpen: () => void
  onDownload: () => void
  onExtract: () => void
  onDelete: () => void
}) {
  const kind = driverFileKind(item.name, item.is_dir)
  const mode = driverOpenMode(kind)
  const openLabel = mode === 'edit' ? 'Editar' : mode === 'view' ? 'Visualizar' : mode === 'folder' ? 'Abrir' : 'Abrir'
  const left = Math.min(x, window.innerWidth - 200)
  const top = Math.min(y, window.innerHeight - 220)
  return (
    <div
      role="menu"
      className="watch-complication fixed z-50 min-w-44 rounded-[16px] border border-white/8 p-1.5"
      style={{ left, top }}
      onClick={(e) => e.stopPropagation()}
    >
      {(mode || kind === 'archive') && (
        <MenuBtn onClick={onOpen} disabled={busy}>
          {mode === 'edit' ? <FilePenLine className="size-4" /> : mode === 'view' ? <Eye className="size-4" /> : <Folder className="size-4" />}
          {kind === 'archive' && !mode ? 'Extrair' : openLabel}
        </MenuBtn>
      )}
      {!item.is_dir && (
        <MenuBtn onClick={onDownload} disabled={busy}>
          <Download className="size-4" />
          Baixar
        </MenuBtn>
      )}
      {kind === 'archive' && (
        <MenuBtn onClick={onExtract} disabled={busy}>
          <Archive className="size-4" />
          {archiveExtractable(item.name) ? 'Extrair aqui' : 'Formato não extraível'}
        </MenuBtn>
      )}
      <MenuBtn onClick={onDelete} disabled={busy} danger>
        <Trash2 className="size-4" />
        Apagar
      </MenuBtn>
    </div>
  )
}

function MenuBtn({
  children,
  onClick,
  disabled,
  danger,
}: {
  children: ReactNode
  onClick: () => void
  disabled?: boolean
  danger?: boolean
}) {
  return (
    <button
      type="button"
      role="menuitem"
      disabled={disabled}
      onClick={onClick}
      className={cn(
        'flex w-full items-center gap-2 rounded-[10px] px-2.5 py-2 text-left text-sm hover:bg-white/8 disabled:opacity-40',
        danger && 'text-destructive',
      )}
    >
      {children}
    </button>
  )
}

import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { MessageCircle, Repeat2, Star } from 'lucide-react'
import { toast } from 'sonner'
import { ProfileLink } from '@/components/profile-link'
import { SocialAvatar } from '@/components/social-avatar'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { api, ApiError, type SocialPost, type SocialPostComment, type SocialPostOriginal } from '@/lib/api'
import { useAuth } from '@/lib/auth-context'
import { useOptionalChat } from '@chat/messenger/ChatProvider'
import { livePresence } from '@/lib/social-presence'
import { formatRelativeTime } from '@/lib/format'
import { cn } from '@/lib/utils'

export function PostCard({ post, onChanged }: { post: SocialPost; onChanged?: () => void }) {
  const { user } = useAuth()
  const chat = useOptionalChat()
  const presence = livePresence(post.author_id, post.presence, chat?.presence)
  const display = post.display_name || post.username
  const targetId = post.kind === 'repost' && post.original ? post.original.id : post.id
  const [starred, setStarred] = useState(post.starred)
  const [stars, setStars] = useState(post.stars)
  const [reposted, setReposted] = useState(post.reposted)
  const [reposts, setReposts] = useState(post.reposts)
  const [comments, setComments] = useState(post.comments)
  const [openComments, setOpenComments] = useState(false)
  const [busy, setBusy] = useState<'star' | 'repost' | null>(null)
  const mine = user?.username === (post.kind === 'repost' && post.original ? post.original.username : post.username)

  async function toggleStar() {
    setBusy('star')
    try {
      const next = await api.starSocialPost(targetId)
      setStarred(next.starred)
      setStars(next.stars)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao marcar estrela')
    } finally {
      setBusy(null)
    }
  }

  async function toggleRepost() {
    if (mine) {
      toast.error('Não dá para repostar o próprio post')
      return
    }
    setBusy('repost')
    try {
      const next = await api.repostSocialPost(targetId)
      setReposted(next.reposted)
      setReposts(next.reposts)
      onChanged?.()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao repostar')
    } finally {
      setBusy(null)
    }
  }

  return (
    <article className="rounded-[18px] border border-white/8 bg-white/[0.03] px-4 py-3.5">
      {post.kind === 'repost' && (
        <p className="mb-2 flex items-center gap-1.5 text-[11px] text-muted-foreground">
          <Repeat2 className="size-3.5" />
          {user?.username === post.username ? 'Você repostou' : `${display} repostou`}
        </p>
      )}
      <div className="flex gap-3">
        <ProfileLink username={post.kind === 'repost' && post.original ? post.original.username : post.username} className="shrink-0">
          <SocialAvatar
            name={post.kind === 'repost' && post.original ? post.original.display_name || post.original.username : display}
            src={post.kind === 'repost' && post.original ? post.original.avatar_url : post.avatar_url}
            presence={post.kind === 'repost' ? undefined : presence}
            className="size-11 text-sm"
          />
        </ProfileLink>
        <div className="min-w-0 flex-1">
          {post.kind === 'repost' && post.original ? (
            <OriginalBlock original={post.original} />
          ) : (
            <>
              <div className="flex flex-wrap items-baseline gap-x-2">
                <ProfileLink username={post.username} className="font-display text-sm font-semibold hover:underline">
                  {display}
                </ProfileLink>
                <span className="text-xs text-muted-foreground">@{post.username}</span>
                <span className="text-xs text-muted-foreground">· {formatRelativeTime(post.created_at)}</span>
              </div>
              <p className="mt-1.5 whitespace-pre-wrap text-sm leading-relaxed">{post.body}</p>
              {post.project_slug && (
                <p className="mt-2 text-[11px] text-muted-foreground">
                  Projeto <span className="font-medium text-foreground/80">{post.project_name || post.project_slug}</span>
                </p>
              )}
            </>
          )}

          <div className="mt-3 flex flex-wrap items-center gap-1">
            <ActionButton
              label={starred ? 'Remover estrela' : 'Marcar estrela'}
              pressed={starred}
              count={stars}
              disabled={busy === 'star'}
              onClick={() => void toggleStar()}
              activeClass="text-[var(--glow-amber)]"
            >
              <Star className={cn('size-4', starred && 'fill-current')} />
            </ActionButton>
            <ActionButton
              label={openComments ? 'Ocultar comentários' : 'Comentar'}
              pressed={openComments}
              count={comments}
              onClick={() => setOpenComments((v) => !v)}
            >
              <MessageCircle className="size-4" />
            </ActionButton>
            <ActionButton
              label={reposted ? 'Desfazer repost' : 'Repostar'}
              pressed={reposted}
              count={reposts}
              disabled={busy === 'repost' || mine}
              onClick={() => void toggleRepost()}
              activeClass="text-[var(--safe)]"
            >
              <Repeat2 className="size-4" />
            </ActionButton>
          </div>

          {openComments && (
            <CommentThread
              postId={targetId}
              onCount={(n) => setComments(n)}
            />
          )}
        </div>
      </div>
    </article>
  )
}

function OriginalBlock({ original }: { original: SocialPostOriginal }) {
  const name = original.display_name || original.username
  return (
    <div className="rounded-[14px] border border-white/8 px-3 py-2.5">
      <div className="flex flex-wrap items-baseline gap-x-2">
        <ProfileLink username={original.username} className="font-display text-sm font-semibold hover:underline">
          {name}
        </ProfileLink>
        <span className="text-xs text-muted-foreground">@{original.username}</span>
        <span className="text-xs text-muted-foreground">· {formatRelativeTime(original.created_at)}</span>
      </div>
      <p className="mt-1.5 whitespace-pre-wrap text-sm leading-relaxed">{original.body}</p>
    </div>
  )
}

function ActionButton({
  label,
  count,
  pressed,
  disabled,
  onClick,
  activeClass,
  children,
}: {
  label: string
  count: number
  pressed?: boolean
  disabled?: boolean
  onClick: () => void
  activeClass?: string
  children: ReactNode
}) {
  return (
    <button
      type="button"
      aria-label={label}
      aria-pressed={pressed}
      title={label}
      disabled={disabled}
      onClick={onClick}
      className={cn(
        'inline-flex min-h-9 items-center gap-1.5 rounded-full px-2.5 text-xs font-medium text-muted-foreground hover:bg-white/6 hover:text-foreground disabled:opacity-40',
        pressed && activeClass,
      )}
    >
      {children}
      <span>{count}</span>
    </button>
  )
}

function CommentThread({ postId, onCount }: { postId: number; onCount: (n: number) => void }) {
  const [items, setItems] = useState<SocialPostComment[] | null>(null)
  const [body, setBody] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let alive = true
    void api
      .listSocialComments(postId, { page: 1, per_page: 40 })
      .then((page) => {
        if (!alive) return
        setItems(page.items)
        onCount(page.total)
      })
      .catch((err) => {
        if (!alive) return
        toast.error(err instanceof ApiError ? err.message : 'Falha ao carregar comentários')
        setItems([])
      })
    return () => {
      alive = false
    }
    // onCount só atualiza o badge; não refaz o fetch
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [postId])

  async function submit(e: FormEvent) {
    e.preventDefault()
    const text = body.trim()
    if (!text) return
    setBusy(true)
    try {
      const created = await api.createSocialComment(postId, text)
      setItems((cur) => {
        const next = [...(cur ?? []), created]
        onCount(next.length)
        return next
      })
      setBody('')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao comentar')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="mt-3 border-t border-white/8 pt-3">
      <ul className="flex flex-col gap-2.5">
        {(items ?? []).map((c) => (
          <li key={c.id} className="flex gap-2">
            <SocialAvatar name={c.display_name || c.username} src={c.avatar_url} className="size-7 text-[10px]" />
            <div className="min-w-0 flex-1">
              <p className="text-xs">
                <ProfileLink username={c.username} className="font-semibold hover:underline">
                  {c.display_name || c.username}
                </ProfileLink>
                <span className="ml-1.5 text-muted-foreground">{formatRelativeTime(c.created_at)}</span>
              </p>
              <p className="mt-0.5 whitespace-pre-wrap text-sm leading-relaxed">{c.body}</p>
            </div>
          </li>
        ))}
        {items?.length === 0 && <li className="text-xs text-muted-foreground">Seja o primeiro a comentar.</li>}
      </ul>
      <form className="mt-3 flex flex-col gap-2" onSubmit={submit}>
        <Textarea
          value={body}
          onChange={(e) => setBody(e.target.value.slice(0, 280))}
          placeholder="Escreva um comentário…"
          rows={2}
        />
        <Button type="submit" size="sm" className="self-end rounded-full" disabled={busy || !body.trim()}>
          {busy ? 'Enviando…' : 'Comentar'}
        </Button>
      </form>
    </div>
  )
}

import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Camera, ImagePlus } from 'lucide-react'
import { toast } from 'sonner'
import { api, ApiError, type SocialProfile } from '@/lib/api'
import { SocialAvatar } from '@/components/social-avatar'
import { SocialBanner } from '@/components/social-banner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'
import {
  attachmentRef,
  BANNER_TONES,
  bannerToneClass,
  parseBannerTone,
  type BannerTone,
} from '@/lib/social-profile-media'

const MAX_PROFILE_IMAGE_BYTES = 8 << 20

export function EditSocialProfileDialog({
  profile,
  open,
  onOpenChange,
  onSaved,
}: {
  profile: SocialProfile
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}) {
  const [displayName, setDisplayName] = useState(profile.display_name)
  const [bio, setBio] = useState(profile.bio)
  const [avatarRef, setAvatarRef] = useState(profile.avatar_url)
  const [bannerRef, setBannerRef] = useState(profile.banner_url)
  const [avatarFile, setAvatarFile] = useState<File | null>(null)
  const [bannerFile, setBannerFile] = useState<File | null>(null)
  const [avatarPreview, setAvatarPreview] = useState<string | undefined>()
  const [bannerPreview, setBannerPreview] = useState<string | undefined>()
  const [saving, setSaving] = useState(false)
  const avatarInput = useRef<HTMLInputElement>(null)
  const bannerInput = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!open) return
    setDisplayName(profile.display_name)
    setBio(profile.bio)
    setAvatarRef(profile.avatar_url)
    setBannerRef(profile.banner_url)
    setAvatarFile(null)
    setBannerFile(null)
    setAvatarPreview(undefined)
    setBannerPreview(undefined)
  }, [open, profile])

  useEffect(() => {
    if (!avatarFile) return
    const url = URL.createObjectURL(avatarFile)
    setAvatarPreview(url)
    return () => URL.revokeObjectURL(url)
  }, [avatarFile])

  useEffect(() => {
    if (!bannerFile) return
    const url = URL.createObjectURL(bannerFile)
    setBannerPreview(url)
    return () => URL.revokeObjectURL(url)
  }, [bannerFile])

  function pickImage(kind: 'avatar' | 'banner', file: File | undefined) {
    if (!file) return
    if (!file.type.startsWith('image/')) {
      toast.error('Escolha uma imagem (JPEG, PNG, WebP ou GIF).')
      return
    }
    if (file.size > MAX_PROFILE_IMAGE_BYTES) {
      toast.error('A imagem precisa ter no máximo 8 MB.')
      return
    }
    if (kind === 'avatar') setAvatarFile(file)
    else {
      setBannerFile(file)
      setBannerRef('')
    }
  }

  function selectTone(tone: BannerTone) {
    setBannerFile(null)
    setBannerPreview(undefined)
    setBannerRef(`tone:${tone}`)
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSaving(true)
    try {
      let nextAvatar = avatarFile ? '' : avatarRef
      let nextBanner = bannerFile ? '' : bannerRef
      if (avatarFile) {
        const att = await api.uploadSocialAttachment(avatarFile)
        nextAvatar = attachmentRef(att.id)
      }
      if (bannerFile) {
        const att = await api.uploadSocialAttachment(bannerFile)
        nextBanner = attachmentRef(att.id)
      }
      await api.patchSocialMe({
        display_name: displayName,
        bio,
        avatar_url: nextAvatar,
        banner_url: nextBanner,
      })
      toast.success('Perfil atualizado')
      onSaved()
      onOpenChange(false)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Falha ao salvar perfil')
    } finally {
      setSaving(false)
    }
  }

  const selectedTone = parseBannerTone(bannerRef)
  const previewName = displayName.trim() || profile.username

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="overflow-hidden p-0 sm:max-w-md" showCloseButton>
        <form onSubmit={handleSubmit}>
          <SocialBanner
            username={profile.username}
            bannerUrl={bannerPreview ?? bannerRef}
            className="h-32"
          >
            <button
              type="button"
              className="absolute inset-0 flex items-center justify-center bg-black/25 text-white opacity-0 transition-opacity hover:opacity-100 focus-visible:opacity-100"
              onClick={() => bannerInput.current?.click()}
            >
              <span className="inline-flex items-center gap-1.5 rounded-full bg-black/50 px-3 py-1.5 text-xs font-medium">
                <ImagePlus className="size-3.5" />
                Foto da capa
              </span>
            </button>
          </SocialBanner>

          <div className="px-5 pb-5">
            <div className="-mt-10 mb-4 flex items-end justify-between gap-3">
              <button
                type="button"
                className="relative rounded-full"
                onClick={() => avatarInput.current?.click()}
                aria-label="Trocar foto do perfil"
              >
                <SocialAvatar
                  name={previewName}
                  src={avatarPreview ?? avatarRef}
                  className="size-[5.5rem] border-4 border-background text-2xl shadow-lg"
                />
                <span className="absolute bottom-1 right-1 flex size-7 items-center justify-center rounded-full bg-primary text-primary-foreground shadow">
                  <Camera className="size-3.5" />
                </span>
              </button>
              {avatarRef || avatarFile ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="rounded-full"
                  onClick={() => {
                    setAvatarFile(null)
                    setAvatarPreview(undefined)
                    setAvatarRef('')
                  }}
                >
                  Usar inicial
                </Button>
              ) : null}
            </div>

            <DialogHeader className="mb-4 text-left">
              <DialogTitle>Editar perfil</DialogTitle>
              <DialogDescription>
                Foto, capa e como você aparece para os outros membros.
              </DialogDescription>
            </DialogHeader>

            <div className="flex flex-col gap-4">
              <div className="flex flex-col gap-2">
                <Label>Capa</Label>
                <div className="flex flex-wrap gap-2">
                  {BANNER_TONES.map((tone) => (
                    <button
                      key={tone}
                      type="button"
                      aria-label={`Tom ${tone}`}
                      aria-pressed={selectedTone === tone && !bannerFile}
                      className={cn(
                        'size-8 rounded-full ring-2 ring-offset-2 ring-offset-background',
                        bannerToneClass(tone),
                        selectedTone === tone && !bannerFile
                          ? 'ring-primary'
                          : 'ring-transparent hover:ring-white/30',
                      )}
                      onClick={() => selectTone(tone)}
                    />
                  ))}
                </div>
              </div>

              <div className="flex flex-col gap-1.5">
                <Label htmlFor="display-name">Nome</Label>
                <Input
                  id="display-name"
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  maxLength={80}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="bio">Bio</Label>
                <Textarea
                  id="bio"
                  value={bio}
                  onChange={(e) => setBio(e.target.value.slice(0, 500))}
                  rows={3}
                />
              </div>
            </div>

            <DialogFooter className="mt-5">
              <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
                Cancelar
              </Button>
              <Button type="submit" className="rounded-full" disabled={saving}>
                {saving ? 'Salvando…' : 'Salvar'}
              </Button>
            </DialogFooter>
          </div>

          <input
            ref={avatarInput}
            type="file"
            accept="image/jpeg,image/png,image/webp,image/gif"
            className="hidden"
            onChange={(e) => {
              pickImage('avatar', e.target.files?.[0])
              e.target.value = ''
            }}
          />
          <input
            ref={bannerInput}
            type="file"
            accept="image/jpeg,image/png,image/webp,image/gif"
            className="hidden"
            onChange={(e) => {
              pickImage('banner', e.target.files?.[0])
              e.target.value = ''
            }}
          />
        </form>
      </DialogContent>
    </Dialog>
  )
}

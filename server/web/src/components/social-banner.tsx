import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { useSocialMediaUrl } from '@/hooks/use-social-media-url'
import { isInlineMediaUrl, parseAttachmentId } from '@/lib/social-profile-media'

export function SocialBanner({
  bannerUrl,
  className,
  children,
}: {
  bannerUrl?: string
  className?: string
  children?: ReactNode
}) {
  const hasPhoto = Boolean(bannerUrl && (parseAttachmentId(bannerUrl) || isInlineMediaUrl(bannerUrl)))
  const photo = useSocialMediaUrl(hasPhoto ? bannerUrl : undefined)

  return (
    <div
      className={cn('relative overflow-hidden', className)}
      style={
        photo
          ? undefined
          : {
              background:
                'linear-gradient(165deg, var(--profile-accent, var(--primary)) 0%, color-mix(in oklch, var(--profile-accent, var(--primary)) 35%, black) 100%)',
            }
      }
    >
      {photo && <img src={photo} alt="" className="absolute inset-0 size-full object-cover" />}
      {children}
    </div>
  )
}

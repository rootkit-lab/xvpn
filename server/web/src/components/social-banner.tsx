import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { useSocialMediaUrl } from '@/hooks/use-social-media-url'
import {
  fallbackBannerClass,
  parseBannerTone,
  bannerToneClass,
} from '@/lib/social-profile-media'

export function SocialBanner({
  username,
  bannerUrl,
  className,
  children,
}: {
  username: string
  bannerUrl?: string
  className?: string
  children?: ReactNode
}) {
  const tone = parseBannerTone(bannerUrl)
  const photo = useSocialMediaUrl(bannerUrl)
  const fallback = tone ? bannerToneClass(tone) : fallbackBannerClass(username)

  return (
    <div className={cn('relative overflow-hidden', photo ? 'bg-black/40' : fallback, className)}>
      {photo && <img src={photo} alt="" className="absolute inset-0 size-full object-cover" />}
      {children}
    </div>
  )
}

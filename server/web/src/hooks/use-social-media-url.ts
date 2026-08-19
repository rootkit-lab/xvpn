import { useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { isInlineMediaUrl, parseAttachmentId } from '@/lib/social-profile-media'

/** Resolve `attachment:<id>` (ou blob local) para URL usável em <img>. */
export function useSocialMediaUrl(ref: string | undefined): string | undefined {
  const [url, setUrl] = useState<string | undefined>(undefined)

  useEffect(() => {
    if (!ref) {
      setUrl(undefined)
      return
    }
    if (isInlineMediaUrl(ref)) {
      setUrl(ref)
      return
    }
    const id = parseAttachmentId(ref)
    if (!id) {
      setUrl(undefined)
      return
    }
    let alive = true
    let objectUrl = ''
    void api
      .fetchSocialAttachment(id)
      .then((blob) => {
        const next = URL.createObjectURL(blob)
        if (!alive) {
          URL.revokeObjectURL(next)
          return
        }
        objectUrl = next
        setUrl(next)
      })
      .catch(() => {
        if (alive) setUrl(undefined)
      })
    return () => {
      alive = false
      if (objectUrl) URL.revokeObjectURL(objectUrl)
    }
  }, [ref])

  return url
}

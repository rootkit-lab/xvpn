import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { profileLocation } from '@/lib/social-profile'

export function ProfileLink({
  username,
  className,
  children,
}: {
  username: string
  className?: string
  children: ReactNode
}) {
  const { href, external } = profileLocation(username)
  if (external) {
    return (
      <a href={href} className={className}>
        {children}
      </a>
    )
  }
  return (
    <Link to={href} className={className}>
      {children}
    </Link>
  )
}

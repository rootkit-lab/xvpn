import { Link } from 'react-router-dom'
import { CircleDot, FolderKanban, Milestone, Tag, AtSign, UserRound, Clock, PenLine } from 'lucide-react'
import { xgitPath } from '@/lib/xgit'
import { cn } from '@/lib/utils'

export type TrackerView = 'issues' | 'assigned' | 'created' | 'mentioned' | 'recent' | 'projects' | 'milestones' | 'labels'

const FILTERS: { id: TrackerView; label: string; icon: typeof CircleDot }[] = [
  { id: 'issues', label: 'Issues', icon: CircleDot },
  { id: 'assigned', label: 'Assigned to me', icon: UserRound },
  { id: 'created', label: 'Created by me', icon: PenLine },
  { id: 'mentioned', label: 'Mentioned', icon: AtSign },
  { id: 'recent', label: 'Recent activity', icon: Clock },
]

const VIEWS: { id: TrackerView; label: string; icon: typeof CircleDot; to: string }[] = [
  { id: 'projects', label: 'Projects', icon: FolderKanban, to: 'projects' },
  { id: 'milestones', label: 'Milestones', icon: Milestone, to: 'milestones' },
  { id: 'labels', label: 'Labels', icon: Tag, to: 'labels' },
]

function issuesHref(slug: string, view: TrackerView) {
  if (view === 'issues') return xgitPath(`${slug}/issues`)
  return xgitPath(`${slug}/issues?view=${view}`)
}

export function XgitTrackerNav({ slug, active }: { slug: string; active: TrackerView }) {
  return (
    <aside className="flex w-full shrink-0 flex-col gap-4 text-sm lg:w-48">
      <nav className="flex flex-col gap-0.5">
        {FILTERS.map((item) => {
          const Icon = item.icon
          const on = active === item.id
          return (
            <Link
              key={item.id}
              to={issuesHref(slug, item.id)}
              className={cn(
                'inline-flex items-center gap-2 rounded-md px-2 py-1.5',
                on ? 'bg-muted/60 text-foreground' : 'text-muted-foreground hover:bg-muted/30 hover:text-foreground',
              )}
            >
              <Icon className="size-3.5" />
              {item.label}
            </Link>
          )
        })}
      </nav>
      <div>
        <p className="hud-label mb-1.5 px-2 text-muted-foreground/70">Views</p>
        <nav className="flex flex-col gap-0.5">
          {VIEWS.map((item) => {
            const Icon = item.icon
            const on = active === item.id
            return (
              <Link
                key={item.id}
                to={xgitPath(`${slug}/${item.to}`)}
                className={cn(
                  'inline-flex items-center gap-2 rounded-md px-2 py-1.5',
                  on ? 'bg-muted/60 text-foreground' : 'text-muted-foreground hover:bg-muted/30 hover:text-foreground',
                )}
              >
                <Icon className="size-3.5" />
                {item.label}
              </Link>
            )
          })}
        </nav>
      </div>
    </aside>
  )
}

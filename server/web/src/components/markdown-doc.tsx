import { type ReactNode } from 'react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

export function MarkdownPreview({ text }: { text: string }) {
  if (!text.trim()) {
    return <p className="text-sm text-muted-foreground">Nada para pré-visualizar.</p>
  }
  return <div className="space-y-2 text-sm leading-relaxed">{renderBlocks(text)}</div>
}

export function MarkdownDoc({
  text,
  label,
  trailing,
  defaultTab = 'md',
}: {
  text: string
  label?: string
  trailing?: ReactNode
  defaultTab?: 'md' | 'txt'
}) {
  return (
    <Tabs defaultValue={defaultTab} className="gap-0">
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border/60 px-3 py-1.5">
        {label ? <p className="hud-label text-muted-foreground/70">{label}</p> : <span />}
        <div className="flex items-center gap-2">
          <TabsList variant="line">
            <TabsTrigger value="md">Markdown</TabsTrigger>
            <TabsTrigger value="txt">Texto</TabsTrigger>
          </TabsList>
          {trailing}
        </div>
      </div>
      <TabsContent value="md" className="p-5">
        <MarkdownPreview text={text} />
      </TabsContent>
      <TabsContent value="txt">
        <pre className="overflow-x-auto p-5 font-mono text-xs leading-relaxed whitespace-pre-wrap">{text}</pre>
      </TabsContent>
    </Tabs>
  )
}

function renderBlocks(text: string): ReactNode[] {
  const lines = text.replace(/\r\n/g, '\n').split('\n')
  const out: ReactNode[] = []
  let i = 0
  let key = 0
  while (i < lines.length) {
    const line = lines[i]
    if (line.startsWith('```')) {
      const buf: string[] = []
      i += 1
      while (i < lines.length && !lines[i].startsWith('```')) {
        buf.push(lines[i])
        i += 1
      }
      if (i < lines.length) {
        i += 1
      }
      out.push(
        <pre key={key++} className="overflow-x-auto rounded-lg bg-muted/40 p-3 font-mono text-xs leading-relaxed">
          <code>{buf.join('\n')}</code>
        </pre>,
      )
      continue
    }
    if (line.startsWith('### ')) {
      out.push(
        <h4 key={key++} className="font-semibold">
          {inlineMd(line.slice(4))}
        </h4>,
      )
    } else if (line.startsWith('## ')) {
      out.push(
        <h3 key={key++} className="text-base font-semibold">
          {inlineMd(line.slice(3))}
        </h3>,
      )
    } else if (line.startsWith('# ')) {
      out.push(
        <h2 key={key++} className="text-lg font-semibold">
          {inlineMd(line.slice(2))}
        </h2>,
      )
    } else if (line.startsWith('- [ ] ')) {
      out.push(
        <p key={key++}>
          ☐ {inlineMd(line.slice(6))}
        </p>,
      )
    } else if (line.startsWith('- [x] ') || line.startsWith('- [X] ')) {
      out.push(
        <p key={key++}>
          ☑ {inlineMd(line.slice(6))}
        </p>,
      )
    } else if (line.startsWith('- ')) {
      out.push(
        <p key={key++}>
          • {inlineMd(line.slice(2))}
        </p>,
      )
    } else if (line.startsWith('> ')) {
      out.push(
        <p key={key++} className="border-l-2 border-border pl-2 text-muted-foreground">
          {inlineMd(line.slice(2))}
        </p>,
      )
    } else {
      out.push(<p key={key++}>{line ? inlineMd(line) : '\u00a0'}</p>)
    }
    i += 1
  }
  return out
}

function inlineMd(raw: string): ReactNode[] {
  const parts = raw.split(/(`[^`]+`|\*\*[^*]+\*\*)/g)
  return parts.map((part, i) => {
    if (part.startsWith('`') && part.endsWith('`') && part.length >= 2) {
      return (
        <code key={i} className="rounded bg-muted/50 px-1 font-mono text-[0.85em]">
          {part.slice(1, -1)}
        </code>
      )
    }
    if (part.startsWith('**') && part.endsWith('**') && part.length >= 4) {
      return <strong key={i}>{part.slice(2, -2)}</strong>
    }
    return <span key={i}>{part}</span>
  })
}

import { type ReactNode } from 'react'
import Markdown, { type Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeRaw from 'rehype-raw'
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { cn } from '@/lib/utils'

const sanitize = {
  ...defaultSchema,
  tagNames: [...(defaultSchema.tagNames ?? []), 'img'],
  attributes: {
    ...defaultSchema.attributes,
    img: ['src', 'alt', 'title', 'width', 'height', 'align'],
    p: [...(defaultSchema.attributes?.p ?? []), ['align']],
    div: [...(defaultSchema.attributes?.div ?? []), ['align']],
  },
}

function safeUrl(url: string): string {
  const u = url.trim()
  if (!u) {
    return ''
  }
  if (/^(https?:|mailto:|#|\/|\.)/i.test(u)) {
    return u
  }
  return ''
}

function stripNode<T extends object>(props: T & { node?: unknown }): Omit<T, 'node'> {
  const { node: _node, ...rest } = props
  return rest
}

function withClass<T extends { className?: string; node?: unknown }>(props: T, extra: string) {
  const rest = stripNode(props)
  return { ...rest, className: cn(extra, rest.className) }
}

const mdComponents: Components = {
  h1: (props) => (
    <h1 {...withClass(props, 'mt-2 mb-4 border-b border-border/50 pb-2 font-display text-2xl font-semibold first:mt-0')} />
  ),
  h2: (props) => (
    <h2 {...withClass(props, 'mt-8 mb-3 border-b border-border/40 pb-1.5 font-display text-xl font-semibold first:mt-0')} />
  ),
  h3: (props) => <h3 {...withClass(props, 'mt-6 mb-2 font-display text-base font-semibold')} />,
  h4: (props) => <h4 {...withClass(props, 'mt-4 mb-2 font-semibold')} />,
  p: (props) => <p {...withClass(props, 'my-3 leading-relaxed [&[align=center]]:text-center')} />,
  a: (props) => {
    const href = props.href
    return (
      <a
        {...withClass(props, 'text-primary underline-offset-2 hover:underline')}
        target={href?.startsWith('http') ? '_blank' : undefined}
        rel={href?.startsWith('http') ? 'noopener noreferrer' : undefined}
      />
    )
  },
  ul: (props) => <ul {...withClass(props, 'my-3 list-disc space-y-1 pl-6')} />,
  ol: (props) => <ol {...withClass(props, 'my-3 list-decimal space-y-1 pl-6')} />,
  li: (props) => <li {...withClass(props, 'leading-relaxed')} />,
  blockquote: (props) => (
    <blockquote {...withClass(props, 'my-3 border-l-2 border-primary/40 pl-3 text-muted-foreground')} />
  ),
  hr: (props) => <hr {...withClass(props, 'my-6 border-border/60')} />,
  strong: (props) => <strong {...withClass(props, 'font-semibold text-foreground')} />,
  code: (props) => (
    <code
      {...withClass(
        props,
        cn(
          'rounded-md bg-muted/60 px-1.5 py-0.5 font-mono text-[0.85em]',
          props.className?.includes('language-') && 'block bg-transparent p-0',
        ),
      )}
    />
  ),
  pre: (props) => (
    <pre {...withClass(props, 'my-4 overflow-x-auto rounded-xl bg-muted/40 p-4 font-mono text-xs leading-relaxed')} />
  ),
  table: (props) => (
    <div className="my-4 overflow-x-auto rounded-xl border border-border/50">
      <table {...withClass(props, 'w-full border-collapse text-sm')} />
    </div>
  ),
  thead: (props) => <thead {...withClass(props, 'bg-muted/30')} />,
  th: (props) => <th {...withClass(props, 'border-b border-border/50 px-3 py-2 text-left font-medium')} />,
  td: (props) => <td {...withClass(props, 'border-b border-border/30 px-3 py-2 align-top')} />,
  img: (props) => (
    <img
      {...withClass(props, 'mx-auto my-4 max-h-48 max-w-[min(100%,16rem)] object-contain')}
      alt={props.alt ?? ''}
      onError={(ev) => {
        ev.currentTarget.style.display = 'none'
      }}
    />
  ),
}

export function MarkdownPreview({ text }: { text: string }) {
  if (!text.trim()) {
    return <p className="text-sm text-muted-foreground">Nada para pré-visualizar.</p>
  }
  return (
    <div className="xgit-md max-w-none text-sm text-foreground/90">
      <Markdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeRaw, [rehypeSanitize, sanitize]]}
        urlTransform={safeUrl}
        components={mdComponents}
      >
        {text}
      </Markdown>
    </div>
  )
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
      <TabsContent value="md" className="p-6">
        <MarkdownPreview text={text} />
      </TabsContent>
      <TabsContent value="txt">
        <pre className="overflow-x-auto p-5 font-mono text-xs leading-relaxed whitespace-pre-wrap">{text}</pre>
      </TabsContent>
    </Tabs>
  )
}

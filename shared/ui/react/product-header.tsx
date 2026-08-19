import type { ReactNode } from 'react'
import { cn } from './cn'
import { ProductMark } from './product-mark'
import { PRODUCT_META, productDisplayName, type ProductId } from './products'

export type { ProductId }

/**
 * Chrome de sistema — ícone + nome do app à esquerda, ações à direita.
 * Sem wordmark ihuull, sem título de rota, sem busca. Página fica no
 * template do app (`PageHeading` / conteúdo).
 */
export function ProductHeader({
  product,
  href = '/',
  trailing,
  className = '',
}: {
  product: ProductId
  href?: string
  trailing?: ReactNode
  className?: string
}) {
  const meta = PRODUCT_META[product]

  return (
    <header
      data-product={product}
      className={cn(
        'chrome-bar relative z-20 flex shrink-0 items-center justify-between gap-3 border-b border-white/8 px-4 py-2.5 md:px-6',
        className,
      )}
    >
      <a href={href} className="flex min-w-0 items-center gap-2.5" aria-label={productDisplayName(product)}>
        <span className="icon-well flex size-8 shrink-0 items-center justify-center rounded-[10px]">
          <ProductMark product={product} className="size-4" />
        </span>
        <span className="min-w-0 leading-tight">
          <span className="font-display block text-[15px] font-semibold tracking-tight">{meta.label}</span>
          {product !== 'ihuull' && (
            <span className="hud-label mt-0.5 block text-muted-foreground/70">{meta.kicker}</span>
          )}
        </span>
      </a>
      {trailing ? <div className="flex shrink-0 items-center gap-2">{trailing}</div> : null}
    </header>
  )
}

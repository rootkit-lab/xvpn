import type { ReactNode } from 'react'
import markUrl from '../brand/ihuull-mark.png'
import wordmarkUrl from '../brand/ihuull-wordmark.png'
import { cn } from './cn'
import { ProductMark } from './product-mark'
import { PRODUCT_META, type ProductId } from './products'

export type { ProductId }

/**
 * Header global ihuull — chrome-bar + mark/wordmark + produto + slot direito.
 * Sem react-router: use `href` / `productHref` (hosts cruzados são origem absoluta).
 */
export function ProductHeader({
  product,
  href = '/',
  productHref,
  children,
  trailing,
  className = '',
}: {
  product: ProductId
  href?: string
  productHref?: string
  children?: ReactNode
  trailing?: ReactNode
  className?: string
}) {
  const meta = PRODUCT_META[product]
  const showProduct = product !== 'ihuull'

  return (
    <header
      data-product={product}
      className={cn(
        'chrome-bar relative z-20 flex shrink-0 items-center gap-3 border-b border-white/8 px-4 py-3 md:gap-4 md:px-6',
        className,
      )}
    >
      <a href={href} className="brand-lockup flex shrink-0 items-center" aria-label="ihuull">
        <img src={markUrl} alt="" className="size-8 object-contain md:hidden" />
        <img
          src={wordmarkUrl}
          alt=""
          className="hidden h-9 w-[7.75rem] object-cover object-[72%_10%] md:block"
        />
      </a>

      {showProduct && (
        <a
          href={productHref ?? href}
          className="flex min-w-0 shrink-0 items-center gap-2"
          aria-label={meta.label}
        >
          <span className="icon-well flex size-8 items-center justify-center rounded-[10px]">
            <ProductMark product={product} className="size-4" />
          </span>
          <span className="min-w-0">
            <span className="font-display block text-[15px] font-semibold tracking-tight">{meta.label}</span>
            <span className="hud-label text-muted-foreground/70">{meta.kicker}</span>
          </span>
        </a>
      )}

      <div className="min-w-0 flex-1">{children}</div>
      {trailing ? <div className="flex shrink-0 items-center gap-2">{trailing}</div> : null}
    </header>
  )
}

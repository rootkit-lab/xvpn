import { cn } from './cn'
import type { ProductId } from './products'

/** Silhueta da fita ihuull (triângulo oco). A cor vem de `--product`. */
export function ProductMark({
  product,
  className = '',
  title,
}: {
  product: ProductId
  className?: string
  title?: string
}) {
  return (
    <svg
      viewBox="0 0 32 32"
      data-product={product}
      className={cn('product-mark', className)}
      role={title ? 'img' : 'presentation'}
      aria-hidden={title ? undefined : true}
      aria-label={title}
    >
      <path
        fill="currentColor"
        fillRule="evenodd"
        d="M16 3.4c.92 0 1.82.28 2.58.82l8.7 6.05A4.35 4.35 0 0 1 29.6 14v8.15a4.35 4.35 0 0 1-2.32 3.73l-8.7 6.05a4.55 4.55 0 0 1-5.16 0l-8.7-6.05A4.35 4.35 0 0 1 2.4 22.15V14c0-1.55.83-2.98 2.32-3.73l8.7-6.05c.76-.54 1.66-.82 2.58-.82zm0 6.2L8.55 14.5v7.15L16 26.6l7.45-4.95V14.5L16 9.6z"
      />
    </svg>
  )
}

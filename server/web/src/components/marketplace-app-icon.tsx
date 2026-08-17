import { Package } from 'lucide-react'
import { ProductMark } from '@xvpn/ui/react/product-mark'
import type { MarketplaceApp } from '@/lib/api'
import { productIdFromCatalogSlug } from '@/lib/marketplace-product'
import { cn } from '@/lib/utils'

/** Ícone do catálogo: arte enviada, senão a fita do produto (nunca caixa genérica). */
export function MarketplaceAppIcon({
  app,
  className,
}: {
  app: Pick<MarketplaceApp, 'slug' | 'icon_url'>
  className?: string
}) {
  if (app.icon_url) {
    return <img src={app.icon_url} alt="" className={cn('object-cover', className)} />
  }
  const product = productIdFromCatalogSlug(app.slug)
  return (
    <span
      data-product={product ?? undefined}
      className={cn('icon-well-lg flex items-center justify-center text-foreground', className)}
    >
      {product ? <ProductMark product={product} className="size-[46%]" /> : <Package className="size-[42%]" />}
    </span>
  )
}

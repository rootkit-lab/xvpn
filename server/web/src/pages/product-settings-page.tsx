import { Link } from 'react-router-dom'
import { ArrowLeft } from 'lucide-react'
import { PRODUCT_META, productDisplayName } from '@xvpn/ui/react/products'

export function ProductSettingsPage({ product }: { product: 'marketplace' | 'xdriver' }) {
  const meta = PRODUCT_META[product]
  const title = productDisplayName(product)

  return (
    <div className="mx-auto flex w-full max-w-xl flex-col gap-6 px-4 py-8">
      <Link to="/" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" />
        {meta.label}
      </Link>
      <div>
        <p className="hud-label text-muted-foreground/70">{meta.kicker}</p>
        <h1 className="font-display mt-1 text-2xl font-semibold tracking-tight">Configurações</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Preferências do {title} neste dispositivo. Senha e perfil ficam no menu da conta.
        </p>
      </div>
      <section className="watch-complication rounded-[18px] p-5">
        <p className="text-sm text-muted-foreground">
          {product === 'marketplace'
            ? 'Downloads usam a sessão atual. Confira o SHA-256 na página do app antes de instalar.'
            : 'Arquivos do Drive só existem dentro da VPN. Pastas pessoais exigem Samba ou SFTP ligados na conta.'}
        </p>
      </section>
    </div>
  )
}

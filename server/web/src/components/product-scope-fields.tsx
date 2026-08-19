import { ALL_PRODUCTS, PRODUCT_DESCRIPTIONS, PRODUCT_LABELS, type Product } from '@/lib/roles'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'

export function ProductScopeFields({
  value,
  onChange,
  disabled,
  hint,
}: {
  value: Product[]
  onChange: (next: Product[]) => void
  disabled?: boolean
  hint?: string
}) {
  function toggle(product: Product, checked: boolean) {
    if (checked) {
      onChange(ALL_PRODUCTS.filter((p) => p === product || value.includes(p)))
      return
    }
    onChange(value.filter((p) => p !== product))
  }

  return (
    <div className="flex flex-col gap-3">
      <div>
        <Label>Escopo de produtos</Label>
        <p className="mt-1 text-xs text-muted-foreground">
          {hint ??
            'Nenhum marcado = admin irrestrito (todas as seções). Marque para limitar a operação àqueles produtos.'}
        </p>
      </div>
      <div className="flex flex-col gap-2">
        {ALL_PRODUCTS.map((product) => (
          <label key={product} className="flex items-start gap-2 text-sm">
            <Checkbox
              checked={value.includes(product)}
              disabled={disabled}
              onCheckedChange={(state) => toggle(product, state === true)}
            />
            <span>
              <span className="font-medium">{PRODUCT_LABELS[product]}</span>
              <span className="block text-xs text-muted-foreground">{PRODUCT_DESCRIPTIONS[product]}</span>
            </span>
          </label>
        ))}
      </div>
    </div>
  )
}

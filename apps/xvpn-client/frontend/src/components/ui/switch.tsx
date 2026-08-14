import { forwardRef, type ButtonHTMLAttributes } from 'react'

// Switch simples em Tailwind puro (sem @radix-ui/react-switch) — evita
// puxar mais uma dependência só para um toggle on/off, ver
// ROADMAP.md Fase 6 (preferências).
export interface SwitchProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'onClick' | 'role'> {
  checked: boolean
  onCheckedChange: (checked: boolean) => void
}

export const Switch = forwardRef<HTMLButtonElement, SwitchProps>(function Switch(
  { checked, onCheckedChange, disabled, className = '', ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onCheckedChange(!checked)}
      className={`relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${
        checked ? 'bg-primary' : 'bg-input'
      } ${className}`}
      {...props}
    >
      <span
        className={`pointer-events-none inline-block h-5 w-5 rounded-full bg-background shadow transition-transform ${
          checked ? 'translate-x-[22px]' : 'translate-x-0.5'
        }`}
      />
    </button>
  )
})

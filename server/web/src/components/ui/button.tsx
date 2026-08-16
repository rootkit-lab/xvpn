import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { Slot } from "radix-ui"

import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "inline-flex shrink-0 items-center justify-center gap-2 font-display text-sm font-medium whitespace-nowrap transition-all outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: "btn-glow rounded-[10px] hover:brightness-105",
        destructive:
          "rounded-[10px] bg-destructive text-white hover:bg-destructive/90 focus-visible:ring-destructive/20 dark:bg-destructive/60 dark:focus-visible:ring-destructive/40",
        outline:
          "rounded-[10px] border border-white/15 bg-transparent shadow-none hover:bg-white/8 hover:text-foreground",
        secondary:
          "icon-well rounded-[10px] text-foreground hover:scale-[1.02] active:scale-[0.98]",
        ghost:
          "rounded-[10px] hover:bg-white/10 hover:text-foreground",
        link: "text-primary underline-offset-4 hover:underline",
        safe: "power-safe rounded-[10px]",
      },
      size: {
        default: "h-9 px-4 py-2 has-[>svg]:px-3",
        xs: "h-6 gap-1 rounded-[8px] px-2 text-xs has-[>svg]:px-1.5 [&_svg:not([class*='size-'])]:size-3",
        sm: "h-8 gap-1.5 rounded-[10px] px-3 has-[>svg]:px-2.5",
        lg: "h-11 rounded-full px-8 has-[>svg]:px-5",
        icon: "icon-well size-8 rounded-[10px]",
        "icon-xs": "size-6 rounded-[8px] [&_svg:not([class*='size-'])]:size-3",
        "icon-sm": "icon-well size-8 rounded-[10px]",
        "icon-lg": "icon-well-lg size-12 rounded-[16px]",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function Button({
  className,
  variant = "default",
  size = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
  }) {
  const Comp = asChild ? Slot.Root : "button"

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }

# Catálogo de componentes — design system ihuull

Fonte da verdade visual: [`scss/_color-system.scss`](./scss/_color-system.scss).  
Efeitos (vidro, glow, Power): [`scss/_utilities.scss`](./scss/_utilities.scss).  
Skill: `desktop-app-ui`. Plano: `PLAN.md` §6.12.

**Não** copie tokens nem reimplemente estas classes num app. Importe daqui.

## Superfícies (CSS)

| Classe | Uso |
|---|---|
| `watch-face` | Fundo da janela / landing / chrome autenticado |
| `watch-vignette` | Overlay (sempre com `ShellFace` ou landing) |
| `watch-complication` | Card / painel / dropdown |
| `watch-complication-lift` | Card da landing (hover sobe + glow) |
| `icon-well` / `icon-well-lg` | Poço de vidro — header (8×8, 10px) e AppSlot (12×12, 16px) |
| `field-glass` | Input / textarea |
| `chrome-bar` | Header e status bar do painel |
| `nav-link` / `nav-link-active` | Item de nav (`--safe` quando ativo) |
| `power-safe` | Botão Power conectado (glow verde + respiração) |
| `btn-glow` | CTA primário (mesmo tratamento, acento `--primary`) |
| `status-safe-dot` | Online / túnel ativo / “meu” |
| `hud-label` | Label de complication — Outfit 10px tracking 0.14em |
| `hud-mono` | Label tipo terminal (opcional) |
| `font-display` | Título Outfit |
| `glow-blob` / `glow-ring` / `text-glow` | Halo do hero / anel de CTA / título |
| `cyber-frame` / `dot-grid` / `scanline` | Decoração HUD; não substituem `watch-face` |

## Primitivos React (`shared/ui/react`)

| Componente | Alias | Quando usar |
|---|---|---|
| `ShellFace` | `@xvpn/ui/react/shell-face` | Todo shell de produto |
| `IconButton` | `@xvpn/ui/react/icon-button` | Ações de ícone (`filled` + `size="lg"` = AppSlot) |
| `Complication` | `@xvpn/ui/react/complication` | Card; props `label`/`value` iguais ao client |
| `StatusDot` | `@xvpn/ui/react/status-dot` | Presença / VPN |

Cascas por app (não duplicar o fundo):

| App | Cascata | Importa |
|---|---|---|
| xvpn-client | `WatchShell` | `ShellFace` |
| xvpn-chat | `ChatShell` | `ShellFace` |
| painel web | `SystemChrome` + landing `/` | `watch-face` no root |

`ChatIconButton` / `WatchIconButton` **reexportam** `IconButton`. Não copie o `className` filled.

## shadcn (por app)

Button, Input, Dialog, DataTable ficam em cada Vite (`components/ui/`) — dependem do React/Tailwind do app. Estilo:

- Card = `watch-complication` + `rounded-[18px]`
- Button default = `btn-glow` + `rounded-[10px]`; `lg` = pill (`rounded-full`); `safe` = `power-safe`
- Input / Textarea = `field-glass` + `rounded-[12px]`
- Ícones de ação = `IconButton` ou `size="icon"` (`icon-well`)

## O que não compartilhar

- Bindings Wails, `VpnService`, helper
- `ChatSidebar` / `ChatPopouts` (só chrome do painel — skill `chat-chrome`)
- Widget SVG dos anéis Power (linguagem e keyframes `watch-ring-*` sim; o SVG fica no client)

## App novo

1. Alias Vite `@xvpn/ui` → `shared/ui`
2. `main.tsx` importa `@xvpn/ui/scss/index.scss` (Sass via Vite — tokens + `watch-*`). `index.css`: `@import "tailwindcss"` + caminho relativo a `shared/ui/css/tailwind-bridge.css`. Não importe o `.scss` de dentro do CSS do Tailwind v4 — o plugin descarta o arquivo e o `@theme` some.
3. Outfit (`@fontsource/outfit` 400/500/600/700)
4. Shell = `ShellFace` ou `watch-face` + `watch-vignette`
5. Skill `new-intranet-app` + esta página

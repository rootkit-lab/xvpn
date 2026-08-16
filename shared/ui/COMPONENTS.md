# Catálogo de componentes — design system ihuull

Fonte da verdade visual: [`scss/_color-system.scss`](./scss/_color-system.scss).  
Skill: `desktop-app-ui`. Plano: `PLAN.md` §6.12.

**Não** copie tokens nem reimplemente `watch-face` / `watch-complication` / `IconButton` num app. Importe daqui.

## Superfícies (CSS)

| Classe | Uso |
|---|---|
| `watch-face` | Fundo da janela / chrome autenticado |
| `watch-vignette` | Overlay (sempre com `ShellFace`) |
| `watch-complication` | Card / painel / dropdown |
| `status-safe-dot` | Online / túnel ativo / “meu” |
| `hud-label` | Label técnica uppercase 10–11px |
| `font-display` | Título Outfit |
| `cyber-frame` | Cantos em L (opcional, HUD) |
| `dot-grid` / `glow-blob` / `scanline` | Decoração; não substituem `watch-face` |

## Primitivos React (`shared/ui/react`)

| Componente | Alias | Quando usar |
|---|---|---|
| `ShellFace` | `@xvpn/ui/react/shell-face` | Todo shell de produto |
| `IconButton` | `@xvpn/ui/react/icon-button` | Ações de ícone (composer, header) |
| `Complication` | `@xvpn/ui/react/complication` | Card de conteúdo |
| `StatusDot` | `@xvpn/ui/react/status-dot` | Presença / VPN |

Cascas por app (não duplicar o fundo):

| App | Cascata | Importa |
|---|---|---|
| xvpn-client | `WatchShell` | `ShellFace` |
| xvpn-chat | `ChatShell` | `ShellFace` |
| painel web | `SystemChrome` | `watch-face` no root |

`ChatIconButton` / `WatchIconButton` **reexportam** `IconButton`. Não copie o `className` filled.

## shadcn (por app)

Button, Input, Dialog, DataTable ficam em cada Vite (`components/ui/`) — dependem do React/Tailwind do app. Estilo:

- Card = `watch-complication` + `rounded-[18px]` (não `rounded-md` navy)
- Botão primário de destaque pode usar `--safe` (conectado / enviar)
- Ícones de ação = `IconButton`, não um terceiro rounded

## O que não compartilhar

- Bindings Wails, `VpnService`, helper
- `ChatSidebar` / `ChatPopouts` (só chrome do painel — skill `chat-chrome`)
- Anéis Power do cliente (linguagem sim; widget não)

## App novo

1. Alias Vite `@xvpn/ui` → `shared/ui`
2. `index.css`: `@import "tailwindcss"` + caminhos relativos a `shared/ui/scss/index.scss` e `shared/ui/css/tailwind-bridge.css` (o resolver do Tailwind v4 não honra alias Vite)
3. Outfit (`@fontsource/outfit` 400/500/600/700)
4. Shell = `ShellFace` ou `watch-face` + `watch-vignette`
5. Skill `new-intranet-app` + esta página

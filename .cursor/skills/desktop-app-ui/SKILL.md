---
name: desktop-app-ui
description: Design system ihuull (painel web, landing, xvpn-client, xvpn-chat e futuros). Use ao criar ou redesenhar UI, WatchShell, SystemChrome, watch-face, complications, tema dark, acento neon, ou quando o usuário pedir o mesmo visual do cliente.
---

# UI ihuull (painel + landing + desktop)

Fonte da verdade: [`shared/ui`](../../../shared/ui/) — SASS (`_color-system.scss` + `_utilities.scss`) + primitivos React. Catálogo: [`shared/ui/COMPONENTS.md`](../../../shared/ui/COMPONENTS.md). Plano: `PLAN.md` §6.12.

Os três Vite **importam** `@xvpn/ui`. **Não** copie tokens nem `watch-face` / `power-safe` / `icon-well` de um `index.css` para outro.

O painel (`/my`, `/admin`, `/social`), a landing (`/`) e os logins **usam este sistema**. Chat no chrome: `inherit` (skill `chat-chrome`) — o `:root` já é ihuull.

## Tokens (não invente outra paleta)

Valores só em `shared/ui/scss/_color-system.scss` (`$dark`).

| Papel | Token | Valor (dark) |
|---|---|---|
| Fundo da janela | `--background` | `oklch(0.11 0.012 260)` |
| Card / painel | `--card` | `oklch(0.18 0.014 260)` |
| Acento primário | `--primary` | azul `oklch(0.72 0.14 230)` |
| Ativo / meu / online | `--safe` / `--glow-safe` | verde neon |
| Tipo | `--font-display` | Outfit |

## Efeitos canônicos (os mesmos do xvpn-client)

| Classe | Onde |
|---|---|
| `watch-face` + `watch-vignette` | Fundo de **qualquer** tela de produto, inclusive landing |
| `watch-complication` | Cards 2×2, feature grid, dropdown |
| `icon-well` / `icon-well-lg` | Botões circulares/squircles do header e AppSlot |
| `power-safe` | Estado conectado / ação “protegido” |
| `btn-glow` | CTA de marca (lista de espera, Entrar) |
| `field-glass` | Inputs |
| `chrome-bar` | Header e status bar |
| `hud-label` | Labels 10px Outfit tracking 0.14em |

Chrome: `ShellFace` / `WatchShell` / `ChatShell` / `SystemChrome`. Botões de ícone: `IconButton` (`filled`). Cards `rounded-[18px]`–`[22px]`.

## Regras

1. App novo = este visual. Tema `icq` / `light` só como opção no messenger.
2. Não copie o SVG dos anéis Power — use `power-safe` + keyframes `watch-ring-*` se precisar do glow.
3. Bolhas/ações do usuário: direita + `--safe`. Conteúdo do outro: esquerda + `watch-complication`.
4. Ações do composer/chamada: `IconButton` filled. Menus = dropdown ancorado (`watch-complication`).
5. Cor nova → `_color-system.scss` na mesma PR. Efeito novo (glow, vidro) → `_utilities.scss`. Proibido segundo `:root` oklch **ou** segundo `.power-safe` no app.
6. Landing não tem paleta própria. Mesmos cards, pills (`size="lg"`), inputs e glow do client.

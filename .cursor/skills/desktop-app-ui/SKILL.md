---
name: desktop-app-ui
description: Design system ihuull (painel web, xvpn-client, xvpn-chat e futuros). Use ao criar ou redesenhar UI, WatchShell, SystemChrome, watch-face, complications, tema dark, acento neon, ou quando o usuário pedir o mesmo visual do cliente.
---

# UI ihuull (painel + desktop)

Fonte da verdade: [`shared/ui`](../../../shared/ui/) — SASS (`_color-system.scss`) + primitivos React. Catálogo: [`shared/ui/COMPONENTS.md`](../../../shared/ui/COMPONENTS.md). Plano: `PLAN.md` §6.12.

Os três Vite **importam** `@xvpn/ui`. **Não** copie tokens nem `watch-face` / `watch-complication` de um `index.css` para outro.

O painel (`server/web`) **usa este sistema** (`/my`, `/admin`, `/social`, logins). Landing pública (`/`) pode ser mais marketing. Chat no chrome: `inherit` (skill `chat-chrome`) — o `:root` já é ihuull.

## Tokens (não invente outra paleta)

Valores só em `shared/ui/scss/_color-system.scss` (`$dark`).

| Papel | Token | Valor (dark) |
|---|---|---|
| Fundo da janela | `--background` | `oklch(0.11 0.012 260)` |
| Card / painel | `--card` | `oklch(0.18 0.014 260)` |
| Acento primário | `--primary` | azul `oklch(0.72 0.14 230)` |
| Ativo / meu / online | `--safe` / `--glow-safe` | verde neon |
| Tipo | `--font-display` | Outfit |

Utilities: `watch-face`, `watch-vignette`, `watch-complication`, `font-display`, `hud-label`. Chrome: `ShellFace` / `WatchShell` / `ChatShell` / `SystemChrome`. Botões de ícone: `IconButton` (`rounded-[10px]` filled). Cards `rounded-[18px]`–`[22px]`.

## Regras

1. App novo = este visual. Tema `icq` / `light` só como opção no messenger.
2. Não copie o botão Power/anéis do client — só a linguagem (tokens, vidro, tipografia, `--safe`).
3. Bolhas/ações do usuário: direita + `--safe`. Conteúdo do outro: esquerda + `watch-complication`.
4. Ações do composer/chamada: `IconButton` filled. Menus = dropdown ancorado.
5. Cor nova → `_color-system.scss` na mesma PR. Proibido segundo `:root` oklch no app.

## Onde não aplicar

Landing marketing (`/`). Logins autenticados **aplicam**.

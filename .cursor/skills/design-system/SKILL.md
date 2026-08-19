---
name: design-system
description: Color system e componentes reutilizáveis ihuull (SASS em shared/ui). Use ao mudar cor, token, watch-face, card, ou ao criar UI no painel/xvpn/xchat.
---

# Design system

1. Leia [`shared/ui/COMPONENTS.md`](../../../shared/ui/COMPONENTS.md) e `PLAN.md` §6.12.
2. Tokens só em `shared/ui/scss/_color-system.scss`.
3. Superfície nova: `ShellFace` ou classes `watch-face` / `watch-complication`. Ícone: `IconButton`.
4. Não adicione `:root { --background: oklch(...) }` em `server/web` nem nos apps.
5. Skill irmã: `desktop-app-ui`.

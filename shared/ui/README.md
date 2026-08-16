# Design system ihuull (`shared/ui`)

Um color system (SASS) e um catálogo de primitivos para **todos** os frontends: painel (`server/web`), `xvpn-client` e `xvpn-chat`. Evita a paleta navy do Workspace e o copy-paste de tokens.

```
shared/ui/
├── scss/_color-system.scss   # maps dark / light / icq
├── scss/_root.scss           # :root
├── scss/_utilities.scss      # watch-* / HUD
├── scss/_themes.scss         # .xvpn-chat-root[data-chat-theme]
├── scss/index.scss
├── css/tailwind-bridge.css   # @theme inline (Tailwind v4)
├── react/                    # ShellFace, IconButton, Complication, StatusDot
└── COMPONENTS.md
```

Mudança de cor → só `_color-system.scss`. Os três apps herdam no próximo build.

Landing pública (`/`), painel autenticado e apps desktop **seguem** este sistema — mesmos efeitos (`power-safe`, `icon-well`, `field-glass`, `watch-complication`). Sem paleta marketing paralela.

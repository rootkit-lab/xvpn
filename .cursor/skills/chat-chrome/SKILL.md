---
name: chat-chrome
description: Invariante do messenger no painel web. Use ao editar ChatDock, ChatSidebar, ChatHost, SystemChrome, PanelStatusBar, ou quando o usuário falar de chat flutuante, FAB, dock, status bar, sidebar, ICQ overlay.
---

# Chat no chrome do painel

O chat **não** é um FAB / popup no canto da tela. Isso compete com o Workspace (navy/azul) e foi revertido.

| Superfície | Onde |
|---|---|
| Gatilho | botão **Chat** na `PanelStatusBar` (badge de não lidas) |
| Painel | `ChatSidebar` no **aside esquerdo** do `SystemChrome` (substitui o nav; Esc volta) |
| Página cheia | `/social/messages` (`Messenger`) |
| Desktop | janela Wails (`apps/xvpn-chat`), temas `icq`/`dark`/`light` |
| Fora | landing, `/my/login`, `/admin/login` |

Tema no chrome web: `inherit` (tokens do painel). Não pinte o dock de verde ICQ sobre o `/my`.

Arquivos: `ChatSidebar.tsx`, `use-chat-panel.ts`, `panel-status-bar.tsx`, `system-chrome.tsx`. Não recrie `ChatDock` flutuante.

Catálogo desktop: skill `marketplace-publish` — sem release `xvpn-chat-v*` o Apps não lista o chat.

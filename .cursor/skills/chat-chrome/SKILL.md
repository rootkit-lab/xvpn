---
name: chat-chrome
description: Invariante do messenger no painel web. Use ao editar ChatSidebar, ChatAccountsBar, ChatConversationModal, ChatHost, SystemChrome, PanelStatusBar, ou quando o usuário falar de chat, FAB, dock, status bar, sidebar, modal, contas.
---

# Chat no chrome do painel

O chat **não** é um FAB / popup no canto da tela e **não** substitui o nav esquerdo.

| Superfície | Onde |
|---|---|
| Gatilho | botão **Chat** na `PanelStatusBar` (badge de não lidas) |
| Contatos | `ChatSidebar` no **aside direito** (só lista; o nav esquerdo permanece) |
| Conversa | `ChatConversationModal` ao clicar um contato |
| Contas | `ChatAccountsBar` na **faixa inferior** (só conversas já existentes) |
| Página cheia | `/social/messages` (`Messenger`) |
| Desktop | janela Wails (`apps/xvpn-chat`), temas `icq`/`dark`/`light` |
| Fora | landing, `/my/login`, `/admin/login` |

Tema no chrome web: `inherit`. Não pinte o dock de verde ICQ sobre o `/my`.

Catálogo desktop: skill `marketplace-publish` — sem release `xvpn-chat-v*` o Apps não lista o chat.

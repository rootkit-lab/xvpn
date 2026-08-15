---
name: chat-chrome
description: Invariante do messenger no painel web. Use ao editar ChatSidebar, ChatAccountsBar, ChatHost, SystemChrome, PanelStatusBar, ou quando o usuário falar de chat, FAB, dock, status bar, sidebar, rail, contas.
---

# Chat no chrome do painel

O chat **não** é um FAB / popup no canto da tela, **não** é um modal sobre o conteúdo e **não** substitui o nav esquerdo.

| Superfície | Onde |
|---|---|
| Gatilho | botão **Chat** na `PanelStatusBar` (à direita, badge de não lidas) |
| Contatos + conversa | `ChatSidebar` no **aside direito** opaco (contatos RTL; conversa acoplada abaixo, sem overlay) |
| Contas | `ChatAccountsBar` no **rodapé do rail** (só conversas já existentes) |
| Página cheia | `/social/messages` (`Messenger`) |
| Desktop | janela Wails (`apps/xvpn-chat`), temas `icq`/`dark`/`light` |
| Fora | landing, `/my/login`, `/admin/login` |

Tema no chrome web: `inherit`. Não pinte o rail de verde ICQ sobre o `/my`. Escape fecha a conversa e depois o rail.

Catálogo desktop: skill `marketplace-publish` — sem release `xvpn-chat-v*` o Apps não lista o chat.

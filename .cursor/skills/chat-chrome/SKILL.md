---
name: chat-chrome
description: Invariante do messenger no painel web. Use ao editar ChatSidebar, ChatPopouts, ChatHost, SystemChrome, PanelStatusBar, ou quando o usuário falar de chat, FAB, dock, status bar, sidebar, rail, facebook, janela, contas.
---

# Chat no chrome do painel

O chat **não** é um FAB / popup no canto da tela, **não** é um modal com backdrop e **não** substitui o nav esquerdo.

| Superfície | Onde |
|---|---|
| Gatilho | botão **Chat** na `PanelStatusBar` (à direita, badge de não lidas) |
| Contatos | `ChatSidebar` no **aside direito** opaco (lista RTL; o nav esquerdo permanece) |
| Conversas abertas | `ChatPopouts` — janelas no **rodapé direito**, estilo Facebook (várias ao mesmo tempo, minimizar vira bolha, sem overlay) |
| Página cheia | `/social/messages` (`Messenger`) |
| Desktop | janela Wails (`apps/xvpn-chat`), temas `icq`/`dark`/`light` |
| Fora | landing, `/my/login`, `/admin/login` |

Tema no chrome web: `inherit`. Não pinte o rail de verde ICQ sobre o `/my`. Escape fecha a janela focada e depois o rail.

Catálogo desktop: skill `marketplace-publish` — sem GitHub Release **com** `.deb`/`.exe` o Apps não lista o chat. Tag do release-please sozinha não dispara `release-chat.yml` (GITHUB_TOKEN); depois do land da PR de release, `workflow_dispatch` com a tag.

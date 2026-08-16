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
| Desktop | janela Wails (`apps/xvpn-chat`) — visual do `xvpn-client` (skill `desktop-app-ui`); temas `dark` (default) / `light` / `icq` |
| Fora | landing, `/my/login`, `/admin/login` |

Tema no chrome web: `inherit` (herda o `:root` do design system — já é preto profundo, não navy). Não pinte o rail de verde ICQ. Escape fecha a janela focada e depois o rail.

Mídia/chamadas (Fase 21): clipe + drag/Ctrl+V + áudio no composer; stories no topo da lista (composer/viewer em **modal**); chamada 1:1 no header da DM. Overlay de chamada vive no `ChatProvider` (uma instância). Settings: sons, teste de mic (loopback), prévia de câmera, privacidade. Ticks enviado/entregue/lido nas bolhas. Sem TURN/porta nova.

Catálogo desktop: skill `marketplace-publish` — sem GitHub Release **com** `.deb`/`.exe` o Apps não lista o chat. Tag do release-please sozinha não dispara `release-chat.yml` (GITHUB_TOKEN); depois do land da PR de release, `workflow_dispatch` com a tag.

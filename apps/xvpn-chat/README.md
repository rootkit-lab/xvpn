# xvpn-chat

Cliente desktop (Wails3) do protocolo social do XVPN (Fase 19.3). Não é um servidor: fala HTTPS/WSS com `vpn.officeempresa.com`, JWT só em memória, sem listener e sem Samba/FileBrowser.

Module path: `github.com/rootkit-lab/xvpn/chat` (disco `apps/xvpn-chat/`) — mesma divergência deliberada do `xvpn-client`.

```bash
cd apps/xvpn-chat
task build    # precisa de wails3 CLI + GTK/WebKit no Linux
./bin/xvpn-chat
```

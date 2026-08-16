# xchat

Cliente desktop (Wails3) do messenger. Não é um servidor: fala HTTPS/WSS com `xchat.corp.ihuull.com` (intranet; exige xvpn conectado). Token **JWE** só em memória, sem listener e sem Samba/FileBrowser.

Module path: `github.com/rootkit-lab/xvpn/chat` (disco `apps/xvpn-chat/`) — mesma divergência deliberada do `xvpn-client`.

```bash
cd apps/xvpn-chat
task build    # precisa de wails3 CLI + GTK/WebKit no Linux
./bin/xvpn-chat
```

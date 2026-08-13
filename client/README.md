<p align="center">
  <img src="../assets/logo.png" alt="XVPN" width="140">
</p>

# xvpn-client

Cliente desktop do XVPN (Windows/Linux): GUI Wails3 (Go + React/Tailwind/shadcn) desacoplada de um **helper privilegiado** que gerencia o túnel WireGuard. Arquitetura completa em [`../PLAN.md`](../PLAN.md) §3.2 e §7; progresso em [`../ROADMAP.md`](../ROADMAP.md) Fase 4.

## Arquitetura em duas partes

O mesmo binário (`xvpn-client`) roda em dois modos, escolhidos pela flag `--helper`:

- **GUI** (padrão, sem privilégio): janela Wails, expõe `VPNService` ao frontend, fala com o helper via IPC. Nunca toca na rede diretamente.
- **Helper** (`xvpn-client --helper`, precisa de `CAP_NET_ADMIN`/root): orquestra `internal/tunnel.Engine` (WireGuard), `internal/apiclient` (enrollment) e `internal/config` (estado persistido do dispositivo). Escuta um socket Unix (Linux, `internal/ipc/transport_linux.go`) ou named pipe (Windows, `transport_windows.go`) restrito a um grupo/usuários autenticados.

Essa separação existe porque a GUI (WebView + libs gráficas) nunca deveria rodar como root — só o helper, que é um processo mínimo sem UI, precisa do privilégio de rede.

### Engine de túnel por plataforma (`internal/platform/`)

| Plataforma | Implementação | Por quê |
|---|---|---|
| Linux (`platform/linux`) | Interface WireGuard nativa do **kernel**, via `netlink` + `wgctrl` (mesma dupla que o `server/`) | Kernel já resolve TUN/crypto/roteamento sem dependência externa |
| Windows (`platform/windows`) | `wireguard-go` (userspace) + driver `wintun` | Windows não tem WireGuard no kernel; é o mesmo motor usado pelo cliente oficial |

## Rodando localmente (desenvolvimento, Linux)

Requer Go 1.25+, Node 18+ e o [CLI do Wails3](https://v3.wails.io/) (`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`) e [`go-task`](https://taskfile.dev/).

```bash
task build   # instala deps do frontend, gera bindings TS e compila o binário em bin/
```

Para rodar a GUI (não precisa de privilégio até tentar conectar de verdade):

```bash
wails3 dev
```

Para rodar só o helper isoladamente (precisa de root/`CAP_NET_ADMIN` para de fato criar a interface):

```bash
sudo ./bin/xvpn-client --helper
```

### Testando sem GUI (headless)

`cmd/devtool-helper` e `cmd/devtool-e2e` existem só para isso: o binário Wails principal linka `libX11`/GTK/WebKit2GTK mesmo em modo `--helper`, o que quebra em ambientes headless (containers, CI). Esses dois comandos isolam helper e cliente IPC sem nenhuma dependência gráfica — úteis para testar enrollment/connect/disconnect/status ponta a ponta em Docker, por exemplo.

```bash
go build -o /tmp/devtool-helper ./cmd/devtool-helper
go build -o /tmp/devtool-e2e ./cmd/devtool-e2e

sudo /tmp/devtool-helper &            # roda o helper em background
/tmp/devtool-e2e enroll https://vpn.exemplo.com CODIGO-DE-CONVITE meu-dispositivo [mtu-opcional]
/tmp/devtool-e2e connect
/tmp/devtool-e2e status
/tmp/devtool-e2e disconnect
```

## MTU (achado da Fase 1 e 4)

O padrão (1420) é seguro na maioria das redes, mas atrás de outra VPN, CGNAT ou rede móvel restritiva pode ocorrer um "PMTU black hole" (handshake do WireGuard funciona, mas tráfego TCP grande — como TLS — trava). O enrollment aceita um MTU customizado opcional (ex.: `1200`) exatamente para esse caso; é um campo avançado na tela de enrollment da GUI.

## IPv6

O servidor sempre inclui `::/0` nas `AllowedIPs` do peer como blackhole anti-vazamento (o túnel só tem endereço/subnet IPv4 — ver `PLAN.md`). Em hosts sem stack IPv6 na interface criada (kernel com `ipv6.disable=1`, ou containers restritos), adicionar essa rota falha; o engine Linux trata isso como best-effort e não derruba a conexão IPv4 — sem IPv6 utilizável já não há vazamento possível de qualquer forma.

## Testes

```bash
go test ./...
go vet ./...
gofmt -l .   # não deve listar nenhum arquivo
```

## Build de produção

```bash
task build                  # Linux, arquitetura atual
task build GOOS=windows     # cross-compile para Windows (ver build/windows/fetch-wintun.ps1 para o driver wintun.dll)
```

Binário resultante em `bin/` — nunca commitado (ver `PLAN.md` §11.1 e `.gitignore`).

## Integração com o desktop (ícone, menu, atalho, autostart) — Linux

`task build` já gera `build/linux/xvpn-client.desktop` (via `wails3 generate .desktop`, configurado em `build/linux/Taskfile.yml`). Depois de instalar o binário em `/usr/local/bin/xvpn-client` (seção abaixo), instale a integração gráfica em nível de usuário — não precisa de `sudo`:

```bash
mkdir -p ~/.local/share/icons/hicolor/128x128/apps ~/.local/share/applications ~/.config/autostart

cp build/appicon.png ~/.local/share/icons/hicolor/128x128/apps/xvpn-client.png
cp build/linux/xvpn-client.desktop ~/.local/share/applications/xvpn-client.desktop   # menu de aplicativos
cp build/linux/xvpn-client.desktop ~/Desktop/xvpn-client.desktop                     # atalho na área de trabalho
cp build/linux/xvpn-client.desktop ~/.config/autostart/xvpn-client.desktop           # abre a GUI no login

chmod +x ~/Desktop/xvpn-client.desktop ~/.local/share/applications/xvpn-client.desktop
gio set ~/Desktop/xvpn-client.desktop metadata::trusted true 2>/dev/null || true   # GNOME/COSMIC: evita aviso de "não confiável"

gtk-update-icon-cache -f -t ~/.local/share/icons/hicolor 2>/dev/null || true
update-desktop-database ~/.local/share/applications 2>/dev/null || true
```

Autostart aqui é só a **GUI** abrir sozinha no login — o helper privilegiado já sobe independentemente disso via `systemctl enable` (seção abaixo). Um autostart configurável de dentro da própria GUI (ligar/desligar numa tela de Configurações) é item da Fase 6 do `ROADMAP.md`; isso aqui é a integração manual/instalador.

## Instalação do helper como serviço (Linux)

O helper roda como serviço de sistema, com o mínimo de privilégio necessário (`CAP_NET_ADMIN`, não root) — ver [`deploy/systemd/xvpn-client-helper.service`](./deploy/systemd/xvpn-client-helper.service). O instalador definitivo (empacotamento `.deb`/AppImage, Fase 7) vai automatizar os passos abaixo; por enquanto, manualmente:

```bash
sudo groupadd -f xvpn
sudo useradd --system --no-create-home --shell /usr/sbin/nologin -g xvpn xvpn-client-helper
sudo usermod -aG xvpn "$USER"   # para a GUI (rodando como você) falar com o helper sem sudo

sudo cp bin/xvpn-client /usr/local/bin/xvpn-client
sudo cp deploy/systemd/xvpn-client-helper.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now xvpn-client-helper

# entra em vigor no novo grupo sem precisar deslogar:
newgrp xvpn
```

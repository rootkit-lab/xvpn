<p align="center">
  <img src="../assets/logo.png" alt="XVPN" width="140">
</p>

# xvpn-server

Control-plane do XVPN: API HTTP + integração WireGuard (via `wgctrl`) + (a partir da Fase 3) o painel web embutido no mesmo binário. Arquitetura completa em [`../PLAN.md`](../PLAN.md) §6; progresso em [`../ROADMAP.md`](../ROADMAP.md).

## Rodando localmente (desenvolvimento)

Requer Go 1.25+ (o `go.mod` já fixa a versão; `go build`/`go test` baixam o toolchain certo automaticamente se necessário).

```bash
export XVPN_JWT_SECRET=$(openssl rand -hex 32)
export XVPN_WG_ENDPOINT=127.0.0.1:51820   # qualquer valor em dev, o enrollment só falha se estiver vazio
go run ./cmd/xvpn-server
```

Sem `CAP_NET_ADMIN`/acesso root, a criação/configuração da interface WireGuard (`EnsureInterface`) vai falhar — isso é esperado em ambiente de desenvolvimento sem privilégio. Para iterar só na API/painel sem tocar em WireGuard de verdade, use os testes (abaixo), que substituem a camada WireGuard por um fake.

### Painel web (`server/web`)

O painel React é embutido no binário via `go:embed` (ver `internal/webui/`). Em desenvolvimento, rode os dois lados separados — o Vite tem proxy configurado para `/api` apontar para `127.0.0.1:8080`:

```bash
# terminal 1 — backend
go run ./cmd/xvpn-server

# terminal 2 — painel com hot-reload
cd web && npm install && npm run dev
```

Sem rodar `npm run build` antes, o binário Go sobe normalmente mas serve uma página de aviso ("painel ainda não foi compilado") em vez do painel — só o `dist/placeholder.txt` está commitado (ver `.gitignore` e `PLAN.md` §11.1).

## Testes

```bash
go test ./...
go vet ./...
gofmt -l .   # não deve listar nenhum arquivo
```

Os testes de `internal/api` usam um `wireguard.PeerManager` fake (não tocam o kernel) e um SQLite em memória — não precisam de privilégio nem de rede.

## Build

```bash
cd web && npm install && npm run build && cd ..   # gera internal/webui/dist/ (embutido no binário)
go build -o bin/xvpn-server ./cmd/xvpn-server
```

Binário resultante em `bin/` — nunca commitado (ver `PLAN.md` §11.1 e `.gitignore`). O passo do painel é obrigatório antes do `go build` sempre que `web/` tiver mudado — o `go:embed` captura o que estiver em `internal/webui/dist/` no momento da compilação.

## Configuração (variáveis de ambiente)

Ver `internal/config/config.go` para a lista completa e valores padrão. Só duas são obrigatórias:

| Variável | Obrigatória | Descrição |
|---|---|---|
| `XVPN_JWT_SECRET` | Sim | Chave do JWE (`dir` + A256GCM). `openssl rand -hex 32`. |
| `XVPN_WG_ENDPOINT` | Sim | `host:porta` público devolvido aos clientes no enrollment (ex.: `206.189.224.72:51820`). |

Veja [`deploy/xvpn-server.env.example`](./deploy/xvpn-server.env.example) para o arquivo completo usado em produção.

## Deploy (produção — VPS)

1. Compilar o painel e o binário: `cd web && npm ci && npm run build && cd .. && go build -o bin/xvpn-server ./cmd/xvpn-server` (no próprio VPS, ou cross-compile local e copiar o binário resultante — o painel já fica embutido nele).
2. Copiar para `/opt/xvpn/bin/xvpn-server` (dono `xvpn:xvpn`).
3. Copiar [`deploy/xvpn-server.env.example`](./deploy/xvpn-server.env.example) para `/opt/xvpn/xvpn-server.env`, preencher os valores reais, `chmod 600`.
4. Garantir que `/etc/wireguard/server.key` (gerado na Fase 1) é legível pelo usuário `xvpn` (`chown xvpn /etc/wireguard/server.key` ou ACL equivalente — nunca `chmod o+r` num arquivo com chave privada).
5. Instalar a unit: copiar [`deploy/systemd/xvpn-server.service`](./deploy/systemd/xvpn-server.service) para `/etc/systemd/system/`, `systemctl daemon-reload`, `systemctl enable --now xvpn-server`.
6. Confirmar: `systemctl status xvpn-server`, `curl https://xvpn.ihuull.com/api/status`, `wg show wg0` (deve refletir os peers já cadastrados no banco, se houver).
7. Instalar o backup: ver [`deploy/xvpn-backup.cron`](./deploy/xvpn-backup.cron) (usa [`deploy/backup.sh`](./deploy/backup.sh), copiado para `/opt/xvpn/bin/backup.sh`).

O server block do Nginx (`xvpn.ihuull.com` → `127.0.0.1:8080`) está em `deploy/nginx/xvpn.conf`. Landing e `*.corp` são arquivos separados.

## Primeiro acesso

No primeiro boot, se não houver nenhum usuário no banco, um admin é criado automaticamente:

- Se `XVPN_ADMIN_USERNAME`/`XVPN_ADMIN_PASSWORD` estiverem definidos no `.env`, esses valores são usados.
- Caso contrário, o usuário `admin` é criado com uma **senha aleatória logada uma única vez** no journal (`journalctl -u xvpn-server | grep -A2 'primeiro usuário admin'`). Troque-a assim que possível.

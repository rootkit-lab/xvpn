# Migração single-node — tudo para `66.29.147.100` (Fase 68)

O VPS `206.189.224.72` (DigitalOcean) será desligado. **Único host de produção:** `66.29.147.100` (BitLaunch `nc-ph-3726`, 878 GiB). Cloudflare (API / painel xadmin) reponta todos os A públicos para o IP novo.

## Estado atual (2026-08-22)

| O quê | Control `.72` | Data `.100` |
|---|---|---|
| Hub WG `10.66.66.1:51820` | sim | peer `10.66.66.2` |
| `xvpn-server` + SQLite | sim (~812 KiB) | — |
| Nginx (público + `*.corp`) | sim | — |
| dnsmasq `10.66.66.1:53` | sim | — |
| Samba `10.66.66.1:445` | sim | — |
| Git bare | NFS do data | disco local ~5 MiB |
| Registry | proxy → data | `10.66.66.2:5000` |
| Docker / codespaces | `cs-apply` local | só registry |
| landpages-ops | `127.0.0.1:3002` | — |
| Disco usado | ~35 GiB / 155 GiB | ~12 GiB / 878 GiB |

## Arquitetura alvo (um nó)

```
66.29.147.100 (eth0 público)
├── wg0 10.66.66.1/24          ← hub (mesmo CIDR; clientes só mudam endpoint)
├── xvpn-server 127.0.0.1:8080 + 10.66.66.1:8080
├── nginx 0.0.0.0:80/443 + 10.66.66.1:443 (*.corp)
├── dnsmasq 10.66.66.1:53
├── smbd 10.66.66.1:445
├── docker (registry + codespaces)
└── /opt/xvpn/data/           ← git, marketplace, social, codespaces, xvpn.db
```

Sem peer mesh `data` separado na mesma máquina — o registro `MeshServer` no SQLite permanece (hostname `data`), mas o processo roda localmente.

## Pré-requisitos

- [ ] Backup completo: `rsync -aH root@206.189.224.72:/opt/xvpn/ /backup/xvpn-pre-single-node/`
- [ ] Token Cloudflare com `Zone:DNS:Edit` (já no painel ou `XVPN_CLOUDFLARE_TOKEN` no env)
- [ ] Chave WG privada do hub exportada (`wg show wg0` / arquivo em `/etc/wireguard/`)
- [ ] Cert `*.corp.ihuull.com` renovável via DNS-01 no host novo
- [ ] Janela de manutenção (~30–60 min downtime VPN + web)

## Fase A — Preparar o `.100` (sem cutover)

Rodar no **data** (`66.29.147.100`). Não altera produção no `.72`.

```sh
apt-get update
apt-get install -y nginx certbot python3-certbot-dns-cloudflare \
  dnsmasq samba nfs-common wireguard wireguard-tools \
  docker.io ufw fail2ban

id xvpn &>/dev/null || useradd -r -s /usr/sbin/nologin -d /opt/xvpn xvpn
install -d -m 0750 -o xvpn -g xvpn /opt/xvpn/{bin,data,log}
```

### Rsync do control (laptop ou do próprio `.100` com chave SSH)

```sh
# Do control → data (exclui git NFS mount — git já está no data)
rsync -aH --info=progress2 \
  --exclude='data/git' \
  root@206.189.224.72:/opt/xvpn/data/ /opt/xvpn/data/

rsync -aH root@206.189.224.72:/opt/xvpn/bin/ /opt/xvpn/bin/
rsync -aH root@206.189.224.72:/etc/xvpn/ /etc/xvpn/
rsync -aH root@206.189.224.72:/etc/nginx/sites-available/ /etc/nginx/sites-available/
rsync -aH root@206.189.224.72:/etc/letsencrypt/ /etc/letsencrypt/
rsync -aH root@206.189.224.72:/etc/samba/ /etc/samba/
rsync -aH root@206.189.224.72:/etc/dnsmasq.d/ /etc/dnsmasq.d/
rsync -aH root@206.189.224.72:/etc/xvpn/dnsmasq-records.hosts /etc/xvpn/
```

Git: já em `/opt/xvpn/data/git` no data — **não** montar NFS.

Registry: mover bind para loopback no mesmo host:

```sh
docker rm -f xvpn-registry
docker run -d --name xvpn-registry --restart unless-stopped \
  -p 127.0.0.1:5000:5000 \
  -v /opt/xvpn/data/registry:/var/lib/registry \
  registry:2
```

Nginx `registry.corp`: `proxy_pass http://127.0.0.1:5000;` (não `10.66.66.2`).

### WireGuard hub (ainda **não** ligar `51820` público)

Copiar config do hub do control (`/etc/wireguard/wg0.conf` ou equivalente gerenciado pelo `xvpn-server`). No cutover, interface sobe com `10.66.66.1/24` e `ListenPort = 51820`.

### landpages-ops

**Fora do single-node** — permanece no `.72` até rehospedar ou cancelar. Ao apagar o `.72`, `ldpops.appapisip.com` deixa de funcionar.

### Smoke local (sem DNS público)

Com `/etc/hosts` apontando `xvpn.ihuull.com` → `127.0.0.1` no próprio `.100`:

```sh
systemctl start xvpn-server   # teste — parar depois se ainda em prep
curl -sf http://127.0.0.1:8080/api/status
```

## Fase B — Cloudflare (API)

Atualizar **todos** os A de `206.189.224.72` → `66.29.147.100`. Manter **DNS only** (sem proxy laranja) em API/WS/VPN.

Registros típicos (`ihuull.com`):

| Nome | Tipo | Conteúdo novo |
|---|---|---|
| `@`, `www` | A | `66.29.147.100` |
| `xvpn`, `marketplace`, `xauth`, `xchat`, `xgroup`, `xdriver` | A | `66.29.147.100` |

Zona `ihuu.com`: `@`, `www` → `66.29.147.100`.

**Não criar** A para `*.corp`. TXT `corp` = `intranet-only` permanece.

### Via API (exemplo)

```sh
# Listar A records atuais
curl -s -H "Authorization: Bearer $CF_TOKEN" \
  "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records?type=A" \
  | jq '.result[] | select(.content=="206.189.224.72") | {id,name,content}'

# Atualizar cada id (PATCH content → 66.29.147.100)
```

Ou: xadmin → **DNS → Zonas** → editar registros (mesma API por baixo).

TTL: reduzir para 300 s **antes** do cutover; restaurar depois.

## Fase C — Cutover (janela)

Ordem recomendada:

1. **Aviso** — usuários reconectam VPN após o passo 4.
2. **Parar** no control: `systemctl stop xvpn-server nginx` (manter wg0 até o passo 5).
3. **Último rsync** incremental `/opt/xvpn/data` e `xvpn.db`.
4. **Cloudflare** — PATCH todos os A → `66.29.147.100`; aguardar propagação (`dig +short xvpn.ihuull.com @1.1.1.1`).
5. **Subir no `.100`:**
   ```sh
   systemctl enable --now xvpn-server nginx dnsmasq smbd docker
   # wg0 hub: 10.66.66.1/24, ListenPort 51820
   ufw allow 22,80,443/tcp; ufw allow 51820/udp
   systemctl reload nginx
   ```
6. **Parar WG no control** — `wg-quick down wg0` ou desligar serviço.
7. **Smoke** (VPN conectada ao endpoint **novo** `66.29.147.100:51820`):
   - `curl -sf https://xvpn.ihuull.com/api/status`
   - `https://xadmin.corp.ihuull.com/admin`
   - `https://xgit.corp.ihuull.com/`
   - `https://xmonitor.corp.ihuull.com/`
   - enroll de device teste
   - Samba `10.66.66.1:445`
8. **Clientes** — atualizar endpoint no painel ou reenroll; `AllowedIPs` inalterados.

## Fase D — Pós-cutover

- [ ] Atualizar `AGENTS.md`, `PLAN.md` §5, `docs/runbooks/cloudflare-dns.md` (IP único)
- [ ] Remover NFS export no data; `umount` no control
- [ ] Mesh `data` no xadmin: IP público `66.29.147.100`; wg peer legado removido do WG
- [ ] Monitorar 7–14 dias; só então cancelar `.72`
- [ ] `git.bak-pre-data-*` no control — apagar após validação

## Rollback

Se falhar antes de desligar o control:

1. Reverter A no Cloudflare → `206.189.224.72`.
2. `systemctl start xvpn-server nginx` no control.
3. Parar serviços no `.100`.
4. Clientes reconectam ao endpoint antigo.

## ufw no host único (alvo)

```sh
ufw default deny incoming
ufw default deny routed
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 51820/udp
ufw route allow in on wg0 out on wan0
ufw route allow in on wan0 out on wg0
# Samba, dnsmasq, Mongo: bind wg0/127.0.0.1 — sem regra eth0
ufw enable
```

### Exit VPN (internet via hub)

No hub, o tráfego de retorno dos clientes (`10.66.80.0/20`) precisa de rota explícita em `wg0`. Sem ela, `ip route get 10.66.80.x` aponta para o gateway WAN e sites públicos (ex. globo.com) não respondem.

```sh
ip route replace 10.66.80.0/20 dev wg0
```

O `xvpn-server` reaplica essa rota no boot (`EnsureReturnRoutes`). NAT de saída no nft deve usar `iifname "wg0" ip saddr <exit-cidr> masquerade` (não `oif != wg0`).

**Teste isolado** (não mexe na sua internet local):

```sh
# no VPS — netns com peer temporário
TEST_IP=10.66.80.240
PRIV=$(wg genkey); PUB=$(echo "$PRIV" | wg pubkey)
wg set wg0 peer "$PUB" allowed-ips "$TEST_IP/32"
ip netns add vpn-test; mkdir -p /etc/netns/vpn-test
echo nameserver 1.1.1.1 > /etc/netns/vpn-test/resolv.conf
ip link add wg-test type wireguard; ip link set wg-test netns vpn-test
ip netns exec vpn-test wg set wg-test private-key <(echo "$PRIV")
ip netns exec vpn-test wg set wg-test peer "$(wg show wg0 public-key)" allowed-ips 0.0.0.0/0 endpoint 127.0.0.1:51820
ip netns exec vpn-test ip addr add "$TEST_IP/32" dev wg-test; ip netns exec vpn-test ip link set wg-test up
ip netns exec vpn-test ip route add default dev wg-test
sleep 2
ip netns exec vpn-test curl -sS ifconfig.me   # deve ser 66.29.147.100
ip netns exec vpn-test curl -sS -o /dev/null -w '%{http_code}\n' -L https://www.globo.com/
wg set wg0 peer "$PUB" remove; ip netns del vpn-test; ip link del wg-test 2>/dev/null || true
```

## O que muda no código/repo (Fase 68.6)

- `AGENTS.md`: IP produção = `66.29.147.100`
- `PLAN.md` §1 / §5: um nó; sem “control vs data” como dois VPS (malha vira “mesmo host” ou fase futura)
- `server/deploy/nginx/corp.conf`: registry `127.0.0.1:5000`
- Probes xmonitor: registry/git locais
- Runbook `data-node-cutover.md`: marcar histórico (NFS era transitório)

## Fora de escopo

- Novo segundo VPS
- Mudar CIDR overlay (`10.66.66.0/24` permanece)
- IPv6

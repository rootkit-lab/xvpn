# Cutover git + registry para o nó `data` (Fase 67.3)

Malha **infra** (`10.66.66.0/24`). Hostnames `*.corp` continuam em `10.66.66.1` — Nginx no **control** faz proxy. Sem SSH control → data (laptop alcança os dois).

## Pré-requisitos

- Peer mesh `data` enrollado (`10.66.66.2/32`, handshake OK).
- Rota `10.66.80.0/20 dev wg0` no hub (`xvpn-overlay-routes.service`).
- A `data.corp.ihuull.com` → IP wg0 do data (dnsmasq).

## 1. Preparar o data (`66.29.147.100`)

```sh
apt-get update && apt-get install -y docker.io nfs-kernel-server
id xvpn &>/dev/null || useradd -r -s /usr/sbin/nologin -d /opt/xvpn xvpn
install -d -m 0750 -o xvpn -g xvpn /opt/xvpn/data/{git,registry,codespaces}
```

### Sync inicial (a partir do control)

No **control** (ou laptop com acesso aos dois):

```sh
rsync -aH --info=progress2 root@206.189.224.72:/opt/xvpn/data/git/     /opt/xvpn/data/git/
rsync -aH --info=progress2 root@206.189.224.72:/opt/xvpn/data/registry/ /opt/xvpn/data/registry/
rsync -aH --info=progress2 root@206.189.224.72:/opt/xvpn/data/codespaces/ /opt/xvpn/data/codespaces/
chown -R xvpn:xvpn /opt/xvpn/data
```

### Registry no data (só wg0)

```sh
systemctl enable --now docker
docker rm -f xvpn-registry 2>/dev/null || true
docker run -d --name xvpn-registry --restart unless-stopped \
  -p 10.66.66.2:5000:5000 \
  -v /opt/xvpn/data/registry:/var/lib/registry \
  registry:2
ss -tlnp | grep ':5000'   # deve ser 10.66.66.2:5000, não 0.0.0.0
```

### Git bare — export NFS (control monta; forge continua no `xvpn-server`)

```sh
grep -q 'data-git' /etc/exports || echo '/opt/xvpn/data/git 10.66.66.1(rw,sync,no_subtree_check,no_root_squash)' >> /etc/exports
exportfs -ra
```

### Firewall no data

```sh
ufw default deny incoming
ufw allow 22/tcp comment 'SSH admin'
ufw allow from 10.66.66.0/24 to any port 5000 proto tcp comment 'registry via infra'
ufw allow from 10.66.66.1 to any port 2049 proto tcp comment 'NFS git from control'
ufw --force enable
```

## 2. Control (`206.189.224.72`)

### Parar registry local

```sh
systemctl disable --now xvpn-registry
```

### Nginx — proxy registry para o data

Em `server/deploy/nginx/corp.conf` (bloco `registry.corp`):

```nginx
proxy_pass http://10.66.66.2:5000;
```

```sh
nginx -t && systemctl reload nginx
```

### Montar git do data (NFS)

```sh
apt-get install -y nfs-common
mv /opt/xvpn/data/git /opt/xvpn/data/git.bak-pre-data-$(date +%Y%m%d) 2>/dev/null || true
mkdir -p /opt/xvpn/data/git
grep -q '10.66.66.2:/opt/xvpn/data/git' /etc/fstab || \
  echo '10.66.66.2:/opt/xvpn/data/git /opt/xvpn/data/git nfs rw,hard,intr,_netdev 0 0' >> /etc/fstab
mount -a
# xvpn-server lê XVPN_GIT_DIR=/opt/xvpn/data/git — sem mudança de binário
```

## 3. Smoke (VPN conectada)

```sh
curl -skI https://xgit.corp.ihuull.com/ | head -1
curl -skI https://registry.corp.ihuull.com/v2/ | head -1
# codespace ativo: abrir cs-<id>.corp no browser
```

## 4. Limpeza no control (só após smoke OK)

```sh
# registry parado; opcional remover imagem local
docker rm -f xvpn-registry 2>/dev/null || true
# git.bak-* pode ficar 7 dias como rollback
```

## Codespaces (67.3 parcial)

`cs-apply` ainda roda no **control** (socket Docker local). Worktrees podem ser rsyncados para o data como backup; containers continuam no control até agente mesh (`cs-apply` remoto) — ver ROADMAP 67.3+.

## Rollback

1. Control: `umount /opt/xvpn/data/git`; restaurar `git.bak-*`; `systemctl enable --now xvpn-registry`; nginx `proxy_pass http://127.0.0.1:5000`.
2. Data: `docker stop xvpn-registry`.

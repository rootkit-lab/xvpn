# Intranet — dnsmasq em `wg0:53` e certificado `*.corp`

Aplicar **depois** do runbook Cloudflare (`docs/runbooks/cloudflare-dns.md`) e da tabela `PLAN.md` §5. Produção: `206.189.224.72`. Passos read-only primeiro (`ss -tulnp`, `ufw status`, `ip -br addr`).

## 1. dnsmasq só na wg0

```bash
# no VPS — read-only
ss -tulnp | grep -E ':53|:443'
ip -br addr show wg0
```

Pacote `dnsmasq`. Conf de referência no repo: `server/deploy/dnsmasq/dnsmasq.conf` → `/etc/dnsmasq.d/xvpn-corp.conf`.

No `/etc/dnsmasq.conf` do Ubuntu, garanta que **não** há `listen-address=0.0.0.0` nem bind em `eth0`. O snippet do repo já tem `bind-interfaces` + `listen-address=10.66.66.1` + `except-interface=eth0`.

```bash
dnsmasq --test
systemctl enable --now dnsmasq
ss -tulnp | grep ':53'
# esperado: 10.66.66.1:53  — NUNCA 0.0.0.0:53 nem o IP público
```

Não abra 53 no ufw público.

## 2. Certificado DNS-01

```bash
# Token Cloudflare com permissão de zona DNS (só no VPS, chmod 600).
# Nunca commitar.
certbot certonly --dns-cloudflare \
  --dns-cloudflare-credentials /root/.secrets/cloudflare.ini \
  -d 'corp.ihuull.com' -d '*.corp.ihuull.com'
```

O wildcard precisa do DNS-01. HTTP-01 não funciona sem A público (e A público para `corp` é proibido).

## 3. Nginx `*.corp`

Cópia: `server/deploy/nginx/corp.conf` → `/etc/nginx/sites-available/xvpn-corp.conf`, `ln -s` em `sites-enabled`.

```bash
nginx -t && systemctl reload nginx
ss -tulnp | grep nginx
# 10.66.66.1:443 além do 0.0.0.0:443 público (xvpn / landing)
```

## 4. Validação

Fora da VPN: `dig +short xchat.corp.ihuull.com @1.1.1.1` → vazio/NXDOMAIN.

Dentro do túnel: `dig +short xchat.corp.ihuull.com @10.66.66.1` → `10.66.66.1`.

`curl -kI https://xchat.corp.ihuull.com` a partir de um peer deve responder; a partir da internet (sem VPN) o TCP em `10.66.66.1` não é roteável.

## 5. DNS no peer

O enrollment devolve `dns: ["10.66.66.1"]`. Dispositivos já enrolled: o helper aplica esse DNS no connect se o estado estiver vazio (default da Fase 23).

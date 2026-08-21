# Container registry (`registry.corp`)

Imagens Docker no estilo GHCR. **Não** é Harbor. Hostname de protocolo — sem app desktop, sem `aud` de marketplace.

## Bind

- Disco: `/opt/xvpn/data/registry`
- Container: `registry:2` via `server/deploy/systemd/xvpn-registry.service`
- Porta: `127.0.0.1:5000` (sem ufw, sem `0.0.0.0`)
- Nginx: `listen 10.66.66.1:443` + `allow 10.66.66.0/24` + `allow 172.17.0.0/16`
- DNS: A `registry.corp.ihuull.com` → `10.66.66.1` (dnsmasq). Sem A público.

## Apply (só no deploy final da Parte XV)

```sh
install -d -m 0750 /opt/xvpn/data/registry
install -m 0644 server/deploy/systemd/xvpn-registry.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now xvpn-registry
# corp.conf já tem o server block; nginx -t && systemctl reload nginx
# DNS: seed DefaultIntranetHosts + dns-apply no xadmin
```

Auth: `docker login registry.corp.ihuull.com` com usuário + JWE. Scope `repository:<org>/<slug>:pull|push` amarra na ACL do Project (`canSeeProject` / developer+).

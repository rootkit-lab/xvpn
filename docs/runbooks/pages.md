# Pages (`pages.corp`)

Blob estático na malha. Path `https://pages.corp.ihuull.com/<org>/<slug>/`. **Não** usar `<org>.pages.corp` (dois rótulos).

## Bind

- Disco: `/opt/xvpn/data/pages/<org>/<slug>/`
- Hostname: `pages.corp.ihuull.com` → `10.66.66.1` (dnsmasq). Sem A público.
- Nginx: `listen 10.66.66.1:443` + `allow 10.66.66.0/24` + `root /opt/xvpn/data/pages`
- Sem porta no ufw. Sem `0.0.0.0`.

## Publish

- Pasta `docs/` ou `public/` do default branch: `POST /api/projects/<org>/<slug>/pages` com `{source: docs|public}`.
- Artifact CI (`pages.tar.gz`): template Deploy Pages / Static HTML. Token só no env (`XVPN_PACKAGES_TOKEN`).
- Wiki: `refs/xgit/wiki` no bare — sem hostname.

## Apply (só no deploy final da Parte XV)

```sh
install -d -m 0750 /opt/xvpn/data/pages
# corp.conf já tem o server block; nginx -t && systemctl reload nginx
# DNS: seed DefaultIntranetHosts + dns-apply no xadmin
```

# Container registry (`registry.corp`)

Imagens Docker no estilo GHCR. **Não** é Harbor. Hostname de protocolo — sem app desktop, sem `aud` de marketplace.

## Bind

- Disco: `/opt/xvpn/data/registry` (nó **data**, `10.66.66.2`)
- Container: `registry:2` no data (`-p 10.66.66.2:5000:5000`)
- Nginx no **control**: `listen 10.66.66.1:443` → `proxy_pass http://10.66.66.2:5000`
- DNS: A `registry.corp.ihuull.com` → `10.66.66.1` (dnsmasq). Sem A público.

## Apply (nó data + proxy no control)

Ver [`data-node-cutover.md`](./data-node-cutover.md). Resumo:

```sh
# data
docker run -d --name xvpn-registry --restart unless-stopped \
  -p 10.66.66.2:5000:5000 \
  -v /opt/xvpn/data/registry:/var/lib/registry registry:2

# control — corp.conf bloco registry.corp
# proxy_pass http://10.66.66.2:5000;
nginx -t && systemctl reload nginx
```

Auth: `docker login registry.corp.ihuull.com` com usuário + JWE. Scope `repository:<org>/<slug>:pull|push` amarra na ACL do Project (`canSeeProject` / developer+).

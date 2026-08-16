# Runbook Cloudflare — DNS público vs intranet (`*.corp`)

Zonas: `ihuull.com` (plataforma) e, se for sua, `ihuu.com` (landing curta). O VPS é `206.189.224.72`. `ldpops.appapisip.com` **não** é desta zona e **não muda**.

Dois planos, de propósito:

| Plano | Onde resolve | Para quê |
|---|---|---|
| Público | Cloudflare → `206.189.224.72` | Landing, painel, enroll, JWT, download do marketplace |
| Intranet | dnsmasq em `10.66.66.1:53` (só `wg0`) | API/WS dos apps (`*.corp.ihuull.com`) |

O cliente WireGuard instala rota `/32` do IP público fora do túnel (`PLAN.md` §6.9). Qualquer HTTPS cujo DNS seja `206.189.224.72` **sai da VPN**. Por isso xchat/xgroup/xdriver **não** usam `wss://…ihuull.com` “de verdade” — usam `*.corp.ihuull.com` → `10.66.66.1`.

## Proxy (nuvem)

Manter **DNS only** (nuvem cinza) em tudo que for API, WebSocket ou VPN. Cloudflare laranja quebra WebSocket longo e o UDP do WireGuard (51820). Landing (`www` / apex) pode ficar laranja depois, se quiser CDN — só se o server block **não** servir `/api` nesse hostname.

## Zona `ihuull.com` — criar

| Tipo | Nome | Conteúdo | Proxy | Para quê |
|---|---|---|---|---|
| A | `@` | `206.189.224.72` | DNS only (laranja só se for landing pura) | `ihuull.com` → landing |
| A | `www` | `206.189.224.72` | igual | landing |
| A | `xvpn` | `206.189.224.72` | **DNS only** | painel, enroll, JWT, marketplace |
| A | `xchat` | `206.189.224.72` | DNS only | **só landing** “conecte a VPN / abra o app” — **não** é o WS |
| TXT | `corp` | `intranet-only` | — | Impede que `corp.ihuull.com` herde um A do wildcard. **Nenhum A** neste nome |
| TXT | `_dmarc` / SPF | (se houver e-mail) | — | opcional |

Certificados públicos (`xvpn`, `www`, `ihuu`): HTTP-01 (Certbot nginx) depois que o A existir.

Certificado `*.corp.ihuull.com`: **DNS-01** com o plugin Cloudflare do Certbot. Não precisa de A público. Guardar o token da API Cloudflare só no VPS (`chmod 600`), nunca no Git.

## Zona `ihuull.com` — NÃO criar

- A/AAAA `corp`, `*.corp`, `xchat.corp`, `xgroup.corp`, `xdriver.corp`
- A `*.ihuull.com` apontando para o VPS **se** isso fizer `corp.ihuull.com` resolver na internet. Se o wildcard já existir: ou apague, ou deixe só como catch-all de nomes **públicos** e cubra `corp` com o TXT acima (mais específico que o wildcard para aquele rótulo)
- Proxy laranja em `xvpn` ou em qualquer hostname que faça upgrade WebSocket
- Registro que publique `10.66.66.1` na internet (não ajuda o cliente fora do túnel e vaza a topologia)

`*.ihuull.com` no Cloudflare casa **um** rótulo (`xvpn.ihuull.com`, `corp.ihuull.com`). **Não** casa `xchat.corp.ihuull.com`. Não conte com o wildcard público para a intranet — a intranet só existe no dnsmasq.

## Zona `ihuu.com` (se o domínio for seu)

| Tipo | Nome | Conteúdo | Proxy |
|---|---|---|---|
| A | `@` | `206.189.224.72` | igual à landing ihuull |
| A | `www` | `206.189.224.72` | igual |

Nginx: `server_name ihuu.com www.ihuu.com` → mesmo root da landing. Sem o A, o hostname não existe.

## DNS interno (obrigatório para apps desktop)

Serviço dnsmasq (ou CoreDNS) em **`10.66.66.1:53` apenas**. Nunca `:53` em `eth0` / `0.0.0.0`.

```
corp.ihuull.com            A    10.66.66.1
*.corp.ihuull.com          A    10.66.66.1
```

Forward do resto (ex. `8.8.8.8`) para não quebrar internet no full-tunnel. O peer WireGuard já pode receber este DNS no enrollment (`DNS = 10.66.66.1`).

Validação:

```bash
# Fora da VPN — deve FALHAR (NXDOMAIN ou sem A)
dig +short xchat.corp.ihuull.com @1.1.1.1

# Dentro da VPN — deve ser 10.66.66.1
dig +short xchat.corp.ihuull.com @10.66.66.1
```

## Checklist rápido (agente / humano)

1. Skill `port-domain-registry-check` — conferir tabela §5 × `ss -tulnp`.
2. Criar só os A da tabela “criar”.
3. Confirmar que `corp` **não** tem A público.
4. Emitir certs públicos (HTTP-01) e `*.corp` (DNS-01) em momentos separados.
5. Server blocks `*.corp` só em `10.66.66.1:443`.
6. ufw público inalterado: `22`, `80`, `443`, `51820/udp`.

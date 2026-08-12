---
name: port-domain-registry-check
description: Verifica se uma nova porta ou serviço a ser adicionado no VPS colide com o registro de portas/domínios do PLAN.md (secão 5) ou com o que já está escutando de fato no servidor. Use antes de configurar um novo serviço de rede, server block do Nginx, ou systemd unit no VPS compartilhado com o landpages-ops.
---

# Checagem de registro de portas/domínios (XVPN)

O VPS `206.189.224.72` hospeda mais de uma aplicação (XVPN e `landpages-ops`). Antes de reservar uma porta interna ou subdomínio novo, confirme que não colide com nada já documentado ou já rodando de fato.

## Uso

```bash
.cursor/skills/port-domain-registry-check/scripts/check.sh [usuario@host]
```

O script:
1. Extrai a tabela de portas/domínios da seção 5 do `PLAN.md` (na raiz do repositório).
2. Roda `ss -tulnp` no servidor via SSH para ver o que está *de fato* escutando.
3. Imprime os dois lado a lado para comparação manual pelo agente.

## Procedimento ao adicionar um serviço novo

1. Rode o script para ver o estado atual (registro documentado x realidade no servidor).
2. Escolha uma porta/subdomínio que não apareça em nenhuma das duas listas.
3. Adicione a nova linha na tabela de `PLAN.md` §5 **antes** de configurar o serviço de fato — o registro deve sempre refletir a intenção, não só o estado após o fato.
4. Depois de configurar o serviço no servidor, rode o script de novo para confirmar que a porta apareceu em `ss -tulnp` exatamente como planejado (nem mais exposta, nem faltando).

## Regras importantes

- Backends internos do XVPN atrás do Nginx devem usar `127.0.0.1:<porta>`, nunca `0.0.0.0:<porta>`.
- Serviços que devem ser acessíveis só pela VPN (Samba, FileBrowser) devem usar `10.66.66.1:<porta>`, nunca `0.0.0.0` nem porta pública via Nginx.
- Não reutilize `8080` (API XVPN), `8081` (FileBrowser) nem `51820/udp` (WireGuard) para outro serviço, mesmo que pareçam livres num teste pontual.

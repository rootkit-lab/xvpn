---
name: wireguard-peer-ops
description: Cria, lista ou remove peers WireGuard manualmente na interface wg0 do VPS do XVPN, para validação da Fase 1 do roadmap ou depuração de conectividade/handshake. Use quando o usuário pedir para testar a VPN manualmente, adicionar um dispositivo de teste sem passar pelo painel web, ou depurar por que um peer não conecta.
---

# Operações manuais de peer WireGuard (XVPN)

Pré-requisito: a interface `wg0` já deve existir no servidor (Fase 1 do `ROADMAP.md` concluída — `ip addr show wg0` deve funcionar). Se não existir, siga a Fase 1 do `ROADMAP.md` primeiro; estes scripts não criam a interface do zero.

Contexto fixo do projeto (não mude sem atualizar `PLAN.md` §5):
- Sub-rede: `10.66.66.0/24`, servidor = `10.66.66.1`
- Endpoint público: `206.189.224.72:51820`

## Scripts disponíveis

### Listar peers e status

```bash
.cursor/skills/wireguard-peer-ops/scripts/list-peers.sh [usuario@host]
```

Mostra `wg show` completo (peers, handshake, transferência) no servidor.

### Adicionar peer de teste

```bash
.cursor/skills/wireguard-peer-ops/scripts/add-test-peer.sh <ip-do-peer, ex: 10.66.66.2> [usuario@host]
```

Gera um par de chaves **local** (na máquina onde o agente está rodando, nunca no servidor — respeita a invariante de `SECURITY.md` de que a chave privada não nasce no servidor), registra a chave pública como peer no servidor via `wg set`, e imprime:
1. A configuração completa (`[Interface]` + `[Peer]`) para colar num cliente WireGuard de teste.
2. O comando exato que foi rodado no servidor, para referência.

A chave privada gerada fica só no terminal local (stdout) — copie e não deixe em arquivo sem necessidade.

### Remover peer

```bash
.cursor/skills/wireguard-peer-ops/scripts/remove-peer.sh <chave-publica-do-peer> [usuario@host]
```

Remove o peer imediatamente da interface `wg0` no servidor.

## Validação de exit node (depois de conectar um peer de teste)

No dispositivo cliente, com o túnel ativo:

```bash
curl ifconfig.me   # deve retornar 206.189.224.72
ping 10.66.66.1     # deve responder — confirma que "estar na mesma rede" funciona
```

Se `curl ifconfig.me` não retornar o IP do servidor, o problema provavelmente é `ip_forward` desabilitado ou regra de MASQUERADE ausente no servidor (ver Fase 1 do `ROADMAP.md`).

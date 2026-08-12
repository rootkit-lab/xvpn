# XVPN

Rede privada própria com exit node via VPS, painel web de administração e cliente desktop multiplataforma — construído em **Go**, **Wails3** e **React + Tailwind + shadcn/ui**.

> Status: **em desenvolvimento** — Fases 0–3 concluídas (hardening do VPS, túnel WireGuard validado, control-plane API em Go rodando em produção, painel web React+Tailwind+shadcn embutido no binário). Veja o [`ROADMAP.md`](./ROADMAP.md) para o checklist de execução e o [`PLAN.md`](./PLAN.md) para a arquitetura completa e as decisões técnicas (com justificativas).

---

## O que é

O XVPN permite que dispositivos (Windows/Linux) entrem em uma rede privada cujo nó central é um VPS próprio, saindo para a internet com o IP público desse servidor (full-tunnel), com:

- **VPN rápida, estável e segura** baseada em WireGuard, com engine embarcada no cliente (sem depender de instalar o WireGuard oficial à parte).
- **Painel web de administração** no VPS para criar/revogar usuários e dispositivos, ver status de conexão e gerenciar compartilhamentos.
- **Cliente desktop** (Windows e Linux) com interface própria (Wails3 + React), que instala, conecta e desconecta com um clique — sem prompts de administrador repetidos.
- **Compartilhamento de arquivos do VPS** de duas formas, ambas restritas à rede privada (nunca expostas na internet pública): unidade de rede via **Samba** e painel web via **FileBrowser**.

## Por que este stack

| Decisão | Por quê | Detalhes |
|---|---|---|
| WireGuard (não OpenVPN/IPSec) | Melhor desempenho, menor superfície de ataque, suporte nativo no kernel Linux e boas libs Go (`wgctrl-go`, `wireguard-go`) | [`PLAN.md` §3.1](./PLAN.md#31-protocolo-de-vpn-wireguard-vs-alternativas) |
| Cliente com helper privilegiado + GUI sem privilégio | Padrão usado por produtos reais (Tailscale, Mullvad); evita prompts de admin repetidos | [`PLAN.md` §3.2](./PLAN.md#32-arquitetura-do-cliente-desktop-onde-fica-o-motor-wireguard) |
| Nginx (não Caddy) como reverse proxy | O VPS também hospeda outra aplicação Go (`landpages-ops`); um único reverse proxy compartilhado evita conflito de porta 80/443 | [`PLAN.md` §3.3](./PLAN.md#33-reverse-proxy-e-tls-nginx-não-caddy) |
| Samba + FileBrowser restritos à interface `wg0` | Compartilhamento de arquivos nunca deve ser alcançável pela internet pública, só pelo túnel | [`PLAN.md` §3.4](./PLAN.md#34-compartilhamento-de-arquivos-samba--filebrowser-ambos-restritos-à-vpn) |
| Chave privada gerada só no cliente | Mesmo um servidor comprometido não permite personificar um dispositivo existente | [`PLAN.md` §3.5](./PLAN.md#35-geração-de-chaves-onde-nasce-a-chave-privada) |

## Infraestrutura

| Item | Valor |
|---|---|
| VPS | Ubuntu 26.04 LTS, IP público `206.189.224.72` |
| Domínio do painel/API | `vpn.officeempresa.com` |
| Sub-rede WireGuard | `10.66.66.0/24` (servidor = `10.66.66.1`) |
| Outra aplicação no mesmo servidor | `landpages-ops` (`ldpops.appapisip.com`) — ver registro de portas em [`PLAN.md` §5](./PLAN.md#5-alocação-de-rede-portas-e-domínios-registro-para-não-colidir-com-landpages-ops) |

## Estrutura do repositório

```
xvpn/
├── PLAN.md              # Arquitetura completa e decisões técnicas justificadas
├── ROADMAP.md            # Checklist de execução por fases
├── README.md             # Este arquivo
├── AGENTS.md              # Instruções para agentes de IA trabalhando neste repo
├── CONTRIBUTING.md        # Convenções de commit, branch, workflow
├── SECURITY.md            # Modelo de ameaças e política de segurança
├── CHANGELOG.md           # Histórico de mudanças
├── .cursor/               # Rules, hooks e skills do Cursor (ver AGENTS.md)
├── server/                # xvpn-server: control-plane API (Go, Fase 2) + painel web React (Fase 3, embutido no binário)
├── client/                # xvpn-client: app desktop Wails3 (Go + React, a criar — Fase 4)
├── shared/                # Tipos/DTOs Go compartilhados (a criar)
└── docs/                  # Documentação complementar (a criar)
```

`client/`, `shared/` e `docs/` ainda não existem — serão criados nas próximas fases do [`ROADMAP.md`](./ROADMAP.md).

## Como começar (para quem for rodar/desenvolver)

O control-plane (`server/`) já existe e está rodando em produção, agora com painel web embutido (Fases 2–3). Ver [`server/README.md`](./server/README.md) para instruções de build/execução/deploy. O cliente desktop (`client/`) ainda não existe.

1. Leia o [`PLAN.md`](./PLAN.md) para entender a arquitetura completa.
2. Acompanhe o progresso em [`ROADMAP.md`](./ROADMAP.md).
3. Se for contribuir, veja [`CONTRIBUTING.md`](./CONTRIBUTING.md).
4. Para entender o modelo de segurança e o que fazer em caso de incidente, veja [`SECURITY.md`](./SECURITY.md).

## Stack tecnológico

- **Servidor**: Go, `wgctrl-go` + `vishvananda/netlink`, Gin, GORM + SQLite, React + Vite + Tailwind v4 + shadcn/ui (painel embutido via `embed.FS`), Nginx + Certbot, Samba, FileBrowser.
- **Cliente**: Go, Wails v3 (beta), React + Tailwind + shadcn/ui, `wireguard-go` + `wgctrl-go`, `wintun` (Windows).
- **Infra**: Ubuntu 26.04, `systemd`, `ufw`, `nftables`/`iptables`, `fail2ban`.

## Licença / visibilidade

Repositório **público** no GitHub (decisão tomada para viabilizar *branch protection* real na `main`, que no plano gratuito do GitHub só está disponível para repositórios públicos em conta pessoal). Segredos, chaves e dados sensíveis nunca são commitados (ver [`SECURITY.md`](./SECURITY.md) e `.gitignore`) — IP e domínios do servidor aparecem na documentação porque fazem parte da arquitetura, não são credenciais. Licença de código ainda não definida.

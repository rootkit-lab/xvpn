# Instruções para agentes de IA — XVPN

Este arquivo contém o contexto que qualquer agente (Cursor, ou outro compatível com o padrão AGENTS.md) precisa ter sempre em mente ao trabalhar neste repositório.

## O que é este projeto

VPN privada própria com exit node via VPS + painel web de administração + cliente desktop (Windows/Linux) em Go/Wails3/React. Arquitetura completa e decisões justificadas estão em [`PLAN.md`](./PLAN.md). Progresso e checklist estão em [`ROADMAP.md`](./ROADMAP.md) — **sempre atualize os checkboxes do `ROADMAP.md`** ao concluir uma tarefa nele listada.

## Fatos que não mudam (não redescubra, não contradiga sem justificar)

- **VPS de produção real**: `206.189.224.72`, Ubuntu 26.04 LTS, acesso root via chave SSH (sem senha configurada de propósito). Trate qualquer comando executado nesse host como produção — não é um ambiente descartável.
- **Sub-rede WireGuard**: `10.66.66.0/24` (servidor = `10.66.66.1`). **Nunca** usar `10.10.0.0/16` ou `10.136.0.0/16` — já estão em uso por outras interfaces/VPC do servidor.
- **Domínios públicos**: `xauth.ihuull.com` (SSO / issuer JWE; cookie `.ihuull.com`), `xvpn.ihuull.com` (portal/enroll; `/admin` só operação), `marketplace.ihuull.com` (loja Play Store, JWE), `xdriver.ihuull.com` (**não** é produto — 444; Drive só em `xdriver.corp`), `www.ihuull.com` / `ihuull.com` e `www.ihuu.com` / `ihuu.com` (landing), `xchat.ihuull.com` (marketing do messenger — **sem** API/WS), `xgroup.ihuull.com` (marketing — **sem** API/WS). `ldpops.appapisip.com` (`landpages-ops`) **não muda**. Todos os A públicos → `206.189.224.72`, **DNS only** em API/WS. Samba e XDriver só em `xdriver.corp` / `wg0`.
- **Intranet** (`*.corp.ihuull.com`): só resolve no DNS interno `10.66.66.1:53` (dnsmasq no `wg0`). Comunicação dos apps desktop: `xchat.corp`, `xgroup.corp`, `xdriver.corp` → `10.66.66.1`. **Nunca** criar A público para `corp` / `*.corp`. Runbook: [`docs/runbooks/cloudflare-dns.md`](./docs/runbooks/cloudflare-dns.md).
- **Nginx é compartilhado** entre o XVPN e o `landpages-ops`. Nunca assuma que o XVPN é o único serviço HTTP do servidor. Antes de reservar uma porta/hostname novo, confira e atualize [`PLAN.md` §5](./PLAN.md#5-alocação-de-rede-portas-e-domínios-registro-para-não-colidir-com-landpages-ops). Server blocks `*.corp` escutam **somente** `10.66.66.1:443`.

## Invariantes de segurança (não negociáveis)

1. **Chave privada WireGuard nunca sai do dispositivo do cliente.** O servidor só recebe e armazena chaves públicas. Nunca implemente um fluxo que gere a chave privada no servidor e a envie ao cliente.
2. **Samba, XDriver, dnsmasq e Mongo nunca são expostos na internet pública.** Samba/dnsmasq escutam só em `wg0` (`10.66.66.1`); XDriver só em `xdriver.corp.ihuull.com` (Nginx `10.66.66.1:443`); Mongo só em `127.0.0.1:27017`. Nunca `0.0.0.0` nem `eth0`. Defesa em profundidade — o firewall não substitui o bind correto.
3. **Firewall é padrão-nega.** Público: `22`, `80`, `443`, `51820/udp`. Sem 53, 445, 27017 na `eth0`. Porta pública nova precisa de justificativa e linha em `PLAN.md` §5.
4. **Nunca commitar segredos** (chaves privadas, tokens, `.env` com credenciais reais) no repositório.
5. **Mudanças de arquitetura relevantes** (troca de biblioteca WireGuard, mudança de sub-rede, novo domínio, etc.) devem ser refletidas no `PLAN.md`, não só no código.
6. **Nunca commitar artefatos de build** (binários, `dist/`, instaladores `.exe`/`.deb`/`.AppImage`) — ver convenção em [`PLAN.md` §11.1](./PLAN.md#111-convenção-de-build-e-artefatos-o-que-é-gerado-onde-fica-é-commitado) e `.gitignore`.
7. **Nunca use `git commit --no-verify`** para contornar o hook `.githooks/pre-commit` sem confirmação explícita do usuário — se o hook bloquear algo que parece um falso positivo, explique o motivo do bloqueio e peça confirmação antes de ignorá-lo.
8. **Nunca commitar diretamente em `main`/`master`.** Toda mudança nasce numa branch (`feat/`, `fix/`, `chore/`, `security/`, `docs/`) e chega em `main` só via Pull Request com squash merge — ver [`CONTRIBUTING.md`](./CONTRIBUTING.md#fluxo-de-trabalho--github-flow-obrigatório-inclusive-solo). Isso vale também para agentes de IA: antes de editar arquivos para uma tarefa não-trivial, crie a branch primeiro (`git checkout -b <tipo>/<descrição>`). O hook `.githooks/pre-commit` bloqueia commits diretos em `main` (exceto merges), e a branch `main` no GitHub tem *branch protection* (exige PR, sem push direto, sem force-push).

## Convenções do repositório

- Documentação e comunicação com o usuário: **português (pt-BR)**. Identificadores de código (variáveis, funções, nomes de pacotes): **inglês**, seguindo convenção idiomática de Go/TypeScript.
- Estrutura planejada do monorepo: `apps/` (produtos distribuíveis), `server/` (control-plane Go + painel React), `shared/` (DTOs Go + `shared/ui` design system SASS), `docs/`. Consulte [`PLAN.md` §11](./PLAN.md#11-estrutura-de-diretórios-monorepo) e [§6.12](./PLAN.md#612-design-system-e-color-system).
- Commits e branches: ver [`CONTRIBUTING.md`](./CONTRIBUTING.md).
- Antes de rodar comandos destrutivos ou que alterem firewall/rede/serviços no VPS, prefira passos read-only primeiro (ex.: `ufw status`, `wg show`, `ss -tulnp`) para confirmar o estado atual antes de alterar algo. Há um hook (`.cursor/hooks.json`) que bloqueia automaticamente padrões claramente destrutivos — não tente contorná-lo sem confirmar explicitamente com o usuário.
- Há dois níveis de hook distintos e complementares: `.cursor/hooks.json` só protege ações do agente de IA dentro do Cursor; `.githooks/pre-commit` protege qualquer `git commit`, de qualquer origem. Um clone novo do repositório precisa rodar `git config core.hooksPath .githooks` uma vez para ativar o segundo (ver `CONTRIBUTING.md`).

## Regras, hooks e skills do Cursor neste repositório

- `.cursor/rules/*.mdc` — convenções específicas por tipo de arquivo (Go do servidor, Go do cliente, frontend React, infraestrutura). São carregadas automaticamente conforme o contexto.
- `.cursor/hooks.json` — bloqueia comandos de shell claramente destrutivos e formata arquivos Go/TS automaticamente após edição.
- `.cursor/skills/` — workflows executáveis para tarefas recorrentes:
  - Infraestrutura: auditoria de segurança do VPS (`vps-security-audit`), operações manuais de peer WireGuard (`wireguard-peer-ops`), checagem de colisão de porta/domínio/corp (`port-domain-registry-check`), deploy do binário (`deploy-xvpn-server`), app de intranet novo (`new-intranet-app`) e notify do xbot (`xbot-notify`).
  - Git/GitHub: criar branch (`start-task`), abrir PR (`ship-pr`), squash-merge (`land-pr`), releases pendentes (`release-status`) e publicação no catálogo (`marketplace-publish`) — ver [`PLAN.md` §13](./PLAN.md#13-versionamento-e-releases).
  - Painel: chrome do chat (`chat-chrome`) — status bar + rail direito (contatos RTL) + janelas de conversa no rodapé (Facebook), nunca FAB nem modal.
  - UI: design system (`desktop-app-ui` / `design-system`) — `shared/ui` SASS, painel = xvpn = xchat; catálogo em `shared/ui/COMPONENTS.md`. Não copiar tokens.
  Use-as em vez de reinventar os mesmos comandos a cada vez.
- **Criação proativa de Skills**: sempre que, numa mesma sessão, um comando ou sequência de passos for repetido 3 ou mais vezes (ou já existir claramente destinado a se repetir no futuro), o agente deve propor ao usuário transformá-lo numa nova Skill em `.cursor/skills/`, seguindo o padrão já estabelecido (`SKILL.md` com frontmatter `name`/`description` + `scripts/`). Não espere o usuário pedir explicitamente — isso mantém o fluxo de trabalho consistente e evita reinventar o mesmo comando de formas ligeiramente diferentes ao longo do tempo.

## Onde encontrar mais detalhe

| Pergunta | Arquivo |
|---|---|
| Por que escolhemos X em vez de Y? | [`PLAN.md`](./PLAN.md) (seção 3, decisões com tabela comparativa) |
| O que falta fazer / em que fase estamos? | [`ROADMAP.md`](./ROADMAP.md) |
| Como contribuir (branch, commit, PR)? | [`CONTRIBUTING.md`](./CONTRIBUTING.md) |
| Modelo de ameaças e resposta a incidentes? | [`SECURITY.md`](./SECURITY.md) |
| O que mudou entre versões? | [`CHANGELOG.md`](./CHANGELOG.md) |

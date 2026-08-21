# TASKS — Nó data + plataforma XGIT

> Branch: `feat/data-node-platform`
> PR: _(abrir com ship-pr)_
> Fase: 66

## Objetivo

Tratar `66.29.147.100` como **nó de dados da malha Compute** (Mongo/git/containers, alivia o `.72`) e garantir o repo **`xcorp/xvpn`** no XGIT como monorepo da plataforma — sem inventário de hosts no forge.

## Contexto

- Control-plane: `206.189.224.72` / `10.66.66.1` (VPN, Nginx, `xvpn-server`).
- Data: `66.29.147.100` → hostname `data`, enroll WireGuard; chave SSH **só no laptop**.
- XGIT: produtos internos como repos (`xcorp/xvpn`, `xvpn-client`, `xchat`). xadmin/xcodespaces são superfícies do monorepo, não VPS.
- Ver `PLAN.md` §6.16 e `docs/areas/compute.md`.

## Checklist

- [x] Remover abordagem `ProjectHost` / inventário no XGIT
- [x] `POST /api/servers/register` (manual, sem BitLaunch; rejeita chave privada)
- [x] Seed `data` (`MeshServer` pending-enroll) + seed `xcorp/xvpn`
- [x] UI Compute: formulário “Cadastrar VPS existente” + coluna origem
- [x] Skill `tasks` + `TASKS.md` nesta branch
- [x] Testes Go passam
- [x] `PLAN.md` §6.16 + Fase 66 no `ROADMAP.md` + `docs/areas/*` + `AGENTS.md`
- [x] Sem segredos no Git

## Fora de escopo

- Mover Mongo/git/Docker de fato para o `.100` (cutover operacional — fases seguintes)
- Providers DigitalOcean/AWS/GCP
- SSH a partir do control-plane
- Esconder menu Serviços / redesign completo do xadmin

## Critério de saída

- Em xadmin → Compute aparece `data` / `66.29.147.100` com enroll_token + bootstrap.
- Repo `xcorp/xvpn` existe no XGIT (restricted/vpn).
- Rodar bootstrap no host via SSH do laptop gera peer em `10.66.66.0/24` (após deploy).

## Notas para o agente

- Skills: `start-task` → trabalho → `ship-pr` → `land-pr` → `deploy-xvpn-server`
- Nunca commit em `main`. Nunca `git commit --no-verify` sem confirmação.

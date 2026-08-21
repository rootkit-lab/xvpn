---
name: tasks
description: Cria ou atualiza TASKS.md amarrando a tarefa atual a um PR (branch + checklist + critério de saída). Use ao iniciar tarefa não-trivial, após start-task, ou quando o usuário pedir TASKS.md / checklist da branch.
---

# TASKS.md (tarefa ↔ PR)

Toda tarefa não-trivial neste repo tem um [`TASKS.md`](../../../TASKS.md) na raiz da **branch de trabalho**. O arquivo é o contrato do PR: o que fazer, o que não fazer, e quando fechar.

## Quando usar

1. Depois de `start-task` (branch `feat/`/`fix/`/`chore/`/`security/`/`docs/`).
2. Quando o usuário pedir "criar TASKS", "checklist da PR", ou "continuar a tarefa".
3. Antes de `ship-pr` — confira se os checkboxes batem com o diff.

## O que o agente faz

1. Confirma que **não** está em `main`/`master`.
2. Cria ou atualiza `TASKS.md` na raiz com o template abaixo (pt-BR).
3. Preenche **Branch**, **Objetivo**, **Fase ROADMAP** (se houver), **Fora de escopo**, **Checklist** e **Critério de saída**.
4. Após abrir o PR (`ship-pr`), atualiza o campo **PR** com a URL.
5. Ao concluir itens, marca `[x]` na mesma branch — não deixe o ROADMAP/TASKS desalinhados do código.

Não invente trabalho fora do pedido do usuário. Se a tarefa mudar de rumo, reescreva o Objetivo e o Fora de escopo antes de continuar.

## Template

```markdown
# TASKS — <título curto>

> Branch: `<tipo>/<slug>`
> PR: _(abrir com ship-pr)_
> Fase: _(ex.: 66 — ou "docs/chore")_

## Objetivo

Uma frase: o que este PR entrega e por quê.

## Contexto

Bullet curto (arquitetura / invariante). Link para `PLAN.md` §… ou `docs/areas/…` se existir.

## Checklist

- [ ] …
- [ ] Testes Go/UI relevantes passam
- [ ] `PLAN.md` / `ROADMAP.md` / docs da área atualizados (se arquitetura)
- [ ] Sem segredos no Git (chave SSH, token BitLaunch, `.env`)

## Fora de escopo

- …

## Critério de saída

Como validar no VPS ou no painel (1–3 bullets).

## Notas para o agente

- Skills: `start-task` → trabalho → `ship-pr` → `land-pr` → `deploy-xvpn-server` se server/
- Nunca commit em `main`. Nunca `git commit --no-verify` sem confirmação.
```

## Scripts

Opcional — scaffold rápido:

```bash
.cursor/skills/tasks/scripts/init-tasks.sh "título curto"
```

## Relação com outros arquivos

| Arquivo | Papel |
|---|---|
| `TASKS.md` | Trabalho **desta branch/PR** |
| `ROADMAP.md` | Fases do produto (longo prazo) |
| `PLAN.md` | Decisões de arquitetura |
| `docs/areas/*` | Runbook/doc por área (compute, xgit, …) |

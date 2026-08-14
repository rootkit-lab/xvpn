# Contribuindo com o XVPN

Este é um projeto de uso pessoal/privado no momento, mas segue convenções de engenharia normais para manter o histórico legível e facilitar retomar o trabalho depois de um tempo parado.

## Configuração inicial (uma vez por clone do repositório)

Este repositório tem um hook de pre-commit versionado em `.githooks/` (fora de `.git/hooks/`, que não é versionado pelo Git). Ative-o logo após clonar:

```bash
git config core.hooksPath .githooks
```

Sem isso, o pre-commit **não roda** nesse clone, e nada bloqueia localmente o commit acidental de segredos/artefatos de build antes do push. Confirme que está ativo com:

```bash
git config core.hooksPath   # deve imprimir ".githooks"
```

## Fluxo de trabalho — GitHub Flow (obrigatório, inclusive solo)

Este projeto segue [GitHub Flow](https://docs.github.com/pt/get-started/using-github/github-flow): `main` é sempre estável e **protegida** — nenhum commit chega lá exceto via merge de Pull Request. Não é uma sugestão informal: há dois níveis de aplicação técnica disso:

1. **Localmente**: `.githooks/pre-commit` bloqueia qualquer `git commit` feito diretamente nas branches `main`/`master` (a única exceção é um merge em andamento, detectado via `MERGE_HEAD`).
2. **No GitHub**: a branch `main` tem *branch protection* configurada — exige Pull Request antes de qualquer mudança chegar nela, não aceita `push` direto nem `force-push`, e só permite squash merge (histórico linear, um commit por PR).

Passo a passo:

1. Confira o [`ROADMAP.md`](./ROADMAP.md) antes de começar — evite trabalhar em algo fora de ordem sem necessidade (ex.: começar o cliente desktop antes de o control-plane da Fase 2 existir gera retrabalho).
2. Atualize sua `main` local: `git checkout main && git pull --ff-only`.
3. Crie uma branch a partir de `main`:
   - `feat/<descrição-curta>` — nova funcionalidade
   - `fix/<descrição-curta>` — correção de bug
   - `chore/<descrição-curta>` — infraestrutura, configuração, tooling, documentação
   - `security/<descrição-curta>` — mudanças relacionadas a hardening/segurança
   - `docs/<descrição-curta>` — documentação pura (sem mudança de código/infra)
4. Faça commits pequenos e coerentes nessa branch (ver convenção de commits abaixo).
5. Atualize o [`ROADMAP.md`](./ROADMAP.md) marcando os checkboxes concluídos **na mesma branch/PR**, não depois.
6. Se a mudança alterar uma decisão de arquitetura documentada no [`PLAN.md`](./PLAN.md), atualize o `PLAN.md` também, na mesma branch.
7. Envie a branch (`git push -u origin <branch>`) e abra um Pull Request (`gh pr create` ou pela interface do GitHub) — mesmo trabalhando sozinho. O PR é onde você revisa o próprio diff completo antes de consolidar; é isso, não a aprovação de terceiros, que dá o valor em uso solo.
8. Faça o merge do PR via **squash merge** (`gh pr merge --squash --delete-branch`, ou o botão equivalente no GitHub) — mantém a `main` com histórico linear, um commit por PR.
9. Sincronize sua `main` local (`git checkout main && git pull --ff-only`) e remova a branch local (`git branch -d <branch>`).

### Por que PR mesmo trabalhando sozinho?

Não é burocracia por burocracia: o PR é o checkpoint onde você olha o diff inteiro de uma vez (não arquivo por arquivo enquanto edita), o que pega erros como uma porta exposta por engano, um `console.log` esquecido, ou um arquivo que não deveria estar ali. A branch protection do GitHub está configurada para exigir PR mas com **0 aprovações obrigatórias** — ou seja, você mesmo pode mergear seu próprio PR, sem depender de outra pessoa.

## Convenção de commits

Seguimos [Conventional Commits](https://www.conventionalcommits.org/):

```
<tipo>(<escopo opcional>): <descrição curta no imperativo>

[corpo opcional explicando o porquê]
```

Tipos usados: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `security`, `perf`.

Exemplos:

```
feat(server): adiciona endpoint de enrollment de dispositivos
fix(client): corrige vazamento de DNS quando kill switch está ativo
security(infra): restringe Samba à interface wg0
docs(plan): atualiza domínio do painel após confirmação de DNS
```

## Versionamento

Cada componente do monorepo (`server`, `client`, `shared`) tem versionamento semântico **independente**, automatizado via [release-please](https://github.com/googleapis/release-please) a partir dos Conventional Commits — ver detalhes completos em [`PLAN.md` §13](./PLAN.md#13-versionamento-e-releases).

O que isso muda no seu dia a dia:

- Como o merge é sempre squash, **o título do PR vira o commit final em `main`** — é ele (não os commits internos da branch) que o `release-please` analisa para decidir o próximo bump de versão (`feat` = minor, `fix` = patch, `!`/`BREAKING CHANGE` = major). Por isso o título do PR também precisa seguir Conventional Commits, com o escopo indicando o componente afetado quando fizer sentido (ex.: `feat(server): ...`, `fix(client): ...`).
- Você não corta versões manualmente: o `release-please` mantém uma PR de release sempre atualizada por componente (label `autorelease: pending`); mergear essa PR é o que publica a tag + GitHub Release + `CHANGELOG.md` do componente. Use a skill `release-status` para consultar o que está pendente.
- O `CHANGELOG.md` da raiz continua sendo só para mudanças "de projeto" (docs, `.cursor/`, workflow de Git, infraestrutura) — não duplica os changelogs por componente.

## Skills do Cursor para este fluxo

Este projeto tem [Agent Skills](https://docs.cursor.com/) (`.cursor/skills/`) que automatizam os passos acima — prefira usá-las a repetir os comandos manualmente:

- **`start-task`** — cria uma branch a partir de `main` atualizada, seguindo a convenção de nome acima.
- **`ship-pr`** — dá push, mostra o checklist abaixo como lembrete, e abre o PR (validando que o título segue Conventional Commits).
- **`release-status`** — mostra as PRs de release pendentes do `release-please` e o changelog de cada uma.

## Antes de abrir PR / finalizar uma tarefa

- [ ] `gofmt`/`go vet` limpos no código Go alterado (o hook `afterFileEdit` do Cursor já formata automaticamente, mas confira antes de commitar)
- [ ] Lint/format do frontend (`eslint`/`prettier`) limpos quando aplicável
- [ ] Nenhum segredo (chave privada, token, senha) commitado — confira `git diff` antes do commit (o hook `.githooks/pre-commit` bloqueia os casos mais óbvios, mas não é infalível)
- [ ] Nenhum artefato de build (binário, `dist/`, instalador) commitado — ver convenção em [`PLAN.md` §11.1](./PLAN.md#111-convenção-de-build-e-artefatos-o-que-é-gerado-onde-fica-é-commitado)
- [ ] Nenhuma porta/serviço novo exposto publicamente sem estar registrado em [`PLAN.md` §5](./PLAN.md#5-alocação-de-rede-portas-e-domínios-registro-para-não-colidir-com-landpages-ops)
- [ ] Checkboxes relevantes do `ROADMAP.md` atualizados
- [ ] Se mexeu em Samba/FileBrowser/firewall no VPS: rodar a skill `vps-security-audit` para confirmar que nada ficou exposto por engano
- [ ] **Título do PR segue Conventional Commits** — ele vira o commit final na `main` (squash merge) e é o que o `release-please` usa para determinar a próxima versão do componente

## Testando localmente

Instruções detalhadas de build/execução serão adicionadas aqui assim que `server/` e `apps/xvpn-client/` existirem (a partir da Fase 2 do roadmap). Por enquanto, qualquer mudança de infraestrutura deve ser validada diretamente no VPS de staging/produção com os comandos read-only primeiro (`ufw status`, `wg show`, `ss -tulnp`) antes de aplicar mudanças.

## Convenção de código

- Comentários e documentação: português.
- Identificadores (variáveis, funções, tipos): inglês, seguindo o idiomático de Go e TypeScript.
- Ver `.cursor/rules/*.mdc` para convenções específicas de cada parte do código (backend Go, cliente Go, frontend React).

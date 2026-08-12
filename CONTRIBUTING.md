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

## Fluxo de trabalho

1. Confira o [`ROADMAP.md`](./ROADMAP.md) antes de começar — evite trabalhar em algo fora de ordem sem necessidade (ex.: começar o cliente desktop antes de o control-plane da Fase 2 existir gera retrabalho).
2. Crie uma branch a partir de `main`:
   - `feat/<descrição-curta>` — nova funcionalidade
   - `fix/<descrição-curta>` — correção de bug
   - `chore/<descrição-curta>` — infraestrutura, configuração, tooling, documentação
   - `security/<descrição-curta>` — mudanças relacionadas a hardening/segurança
3. Faça commits pequenos e coerentes (ver convenção abaixo).
4. Atualize o [`ROADMAP.md`](./ROADMAP.md) marcando os checkboxes concluídos **no mesmo commit/PR**, não depois.
5. Se a mudança alterar uma decisão de arquitetura documentada no [`PLAN.md`](./PLAN.md), atualize o `PLAN.md` também.
6. Abra PR (ou, em uso solo, faça merge direto após revisão própria) com uma descrição curta do "porquê", não só do "o quê".

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

## Antes de abrir PR / finalizar uma tarefa

- [ ] `gofmt`/`go vet` limpos no código Go alterado (o hook `afterFileEdit` do Cursor já formata automaticamente, mas confira antes de commitar)
- [ ] Lint/format do frontend (`eslint`/`prettier`) limpos quando aplicável
- [ ] Nenhum segredo (chave privada, token, senha) commitado — confira `git diff` antes do commit (o hook `.githooks/pre-commit` bloqueia os casos mais óbvios, mas não é infalível)
- [ ] Nenhum artefato de build (binário, `dist/`, instalador) commitado — ver convenção em [`PLAN.md` §11.1](./PLAN.md#111-convenção-de-build-e-artefatos-o-que-é-gerado-onde-fica-é-commitado)
- [ ] Nenhuma porta/serviço novo exposto publicamente sem estar registrado em [`PLAN.md` §5](./PLAN.md#5-alocação-de-rede-portas-e-domínios-registro-para-não-colidir-com-landpages-ops)
- [ ] Checkboxes relevantes do `ROADMAP.md` atualizados
- [ ] Se mexeu em Samba/FileBrowser/firewall no VPS: rodar a skill `vps-security-audit` para confirmar que nada ficou exposto por engano

## Testando localmente

Instruções detalhadas de build/execução serão adicionadas aqui assim que `server/` e `client/` existirem (a partir da Fase 2 do roadmap). Por enquanto, qualquer mudança de infraestrutura deve ser validada diretamente no VPS de staging/produção com os comandos read-only primeiro (`ufw status`, `wg show`, `ss -tulnp`) antes de aplicar mudanças.

## Convenção de código

- Comentários e documentação: português.
- Identificadores (variáveis, funções, tipos): inglês, seguindo o idiomático de Go e TypeScript.
- Ver `.cursor/rules/*.mdc` para convenções específicas de cada parte do código (backend Go, cliente Go, frontend React).

# teste

Playground do **XCODESPACES** — repo pequeno no XGIT para validar o IDE remoto (`cs-*.corp`), não o monorepo XVPN.

Abre em `https://xgit.corp.ihuull.com/teste`. O clone no container fica em `/home/workspace/project`.

## Checklist

Marque na primeira sessão (Welcome deve ser **XCODESPACES**, não *Get Started with VS Code for the Web*):

1. **Explorer** mostra `cmd/`, `web/`, `scripts/`, `.devcontainer/` — sem `.cache` nem `.openvscode-server`
2. **Tema** ihuull Dark (fundo quase preto, acento violeta)
3. **Terminal** (`Ctrl+\``):
   ```bash
   go version
   node -v
   ./scripts/check.sh
   ```
4. **Source Control** lista só arquivos deste repo (nada do HOME do IDE)
5. **Tasks** → Run Task → `check` (Go test + Node)
6. **ENVs** (Settings do XGIT → Codespaces): grave `TESTE_WHO` e `TESTE_MARK` (plaintext). Depois do Recreate, `node web/index.mjs` imprime os valores. `XCS_LLM_*` **não** aparece no container.
7. Branch + commit + push para `xgit.corp` (developer não empurra `main` direto)

## Layout

| Path | Para quê |
|---|---|
| `cmd/hello` | Go no terminal (`go test ./...`) |
| `web/` | Node sem dependências (`node web/index.mjs`) |
| `.devcontainer/` | Imagem `ihuull/codespace:1.98.2` + settings do tema |
| `.vscode/tasks.json` | Tasks do IDE (commitadas de propósito) |

Fonte canônica no monorepo: `server/deploy/codespace/sample-teste/`. Re-semear o bare: `server/deploy/codespace/seed-teste.sh`.

# Agente no playground `teste`

Repo pequeno do XGIT para validar o IDE remoto (`cs-*.corp`). Não é o monorepo XVPN.

- Conventional Commits (`feat`/`fix`/`docs`/`chore`).
- Não commitar em `main` — crie `feat/` ou `fix/` e abra PR no XGIT.
- Source Control usa `user.name`/`user.email` do dono do codespace (`username@corp.ihuull.com`).
- Tools: `read_file`, `list_dir`, `grep`, `glob`, `write_file`, `apply_patch`, `run_terminal` (allowlist + **python3** + `env` + espera), `list_mcp` / `call_mcp`.
- Terminal **não é shell**: `TESTE_WHO=Agente python3` falha — use `env: {TESTE_WHO: Agente}` e `argv: ["python3", ...]`.
- Sem `docker.sock`, sem Copilot/Continue. Este clone veio do **xgit.corp**, não do GitHub.

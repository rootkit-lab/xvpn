# Git do projeto

O Source Control lista só o clone (`/home/workspace/project`).

Não devem aparecer:

- `.cache/` (Microsoft / openvscode)
- `.openvscode-server/` (logs, lock, extensões)
- `settings.json` gerado pelo IDE no HOME

Se ainda vir esses arquivos, o codespace nasceu com o clone no HOME — **Delete + Create** no XCODESPACES aplica o mount novo (start de container antigo não troca o `-v`).

Commit e push vão para `xgit.corp` com o token curto do codespace. `main` protegida: developer abre branch + PR.

No repo **`teste`**, o `.gitignore` e as tasks em `.vscode/tasks.json` servem para validar Explorer, SCM e Terminal.

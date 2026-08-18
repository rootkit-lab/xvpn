# Git do projeto

O Source Control lista só o clone (`/home/workspace/project`).

Não devem aparecer:

- `.cache/` (Microsoft / openvscode)
- `.openvscode-server/` (logs, lock, extensões)
- `settings.json` gerado pelo IDE no HOME

Se ainda vir esses arquivos, o codespace nasceu com o clone no HOME — **Recreate** no XCODESPACES aplica o mount novo.

Commit e push vão para `xgit.corp` com o token curto do codespace. `main` protegida: developer abre branch + PR.

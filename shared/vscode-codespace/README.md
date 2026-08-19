# ihuull.codespace

Extensão bakeada na imagem `ihuull/codespace`. **Chat** à direita no container `workbench.panel.chat` (OpenVSCode 1.98 não aceita `secondarySidebar`) e **Generate commit message** no Source Control.

O proxy é o `xvpn-server` (`GET /api/xcodespaces/llm/models`, `POST /api/xcodespaces/llm/*`). A extensão é `extensionKind: workspace` (Node): usa URL absoluta `https://cs-<id>.corp.ihuull.com` e o token Git de `.git/xvpn-credentials`. Chrome: modos Agent / Ask / Debug / Plan e seletor de modelo (catálogo do provedor no xadmin). Grava `user.name`/`user.email` do dono no clone (Source Control). Lê `AGENTS.md` (ou contrato ihuull), `CONTRIBUTING.md`, `.cursor/skills` e `.cursor/rules`. Tools (read/glob/edit/term) só no clone, com confirmação. Provedor e key ficam em **xadmin → Settings**. Sem Copilot, sem Continue, sem `docker.sock`.

# ihuull.codespace

Extensão bakeada na imagem `ihuull/codespace`. **Agente** no activity bar (não o Chat/Copilot nativo) e **Generate commit message** no Source Control.

O proxy é o `xvpn-server` (`POST /api/xcodespaces/llm/*`). A extensão é `extensionKind: workspace` (Node): usa URL absoluta `https://cs-<id>.corp.ihuull.com` e o token Git de `.git/xvpn-credentials`. Lê `AGENTS.md`, `.cursor/skills` e `.cursor/rules`. Tools (read/edit/term) só no clone, com confirmação. Provedor e key ficam em **xadmin → Settings**. Sem Copilot, sem Continue, sem `docker.sock`.

---
name: mcp
description: Servidores MCP do codespace (think, memory, docs). Use list_mcp e call_mcp para raciocinar, lembrar e puxar documentação.
---

# MCP no XCODESPACES

1. `list_mcp` — think, memory, docs (bakeados).
2. `call_mcp` com `server`, `name`, `arguments`.

| Server | Tools |
|---|---|
| think | `think` |
| memory | `memory_add`, `memory_get` |
| docs | `fetch_docs` (https allowlisted) |

Extra: `.cursor/mcp.json` só `python3` + `.cursor/mcp/*.py`. Sem Mongo MCP.

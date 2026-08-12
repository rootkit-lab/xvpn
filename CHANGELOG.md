# Changelog

Todas as mudanças relevantes deste projeto são documentadas aqui.

Formato baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.0.0/), versionamento seguirá [SemVer](https://semver.org/lang/pt-BR/) a partir da primeira release.

## [Unreleased]

### Added

- `PLAN.md` com arquitetura completa do projeto, diagnóstico real do VPS e decisões técnicas justificadas.
- `ROADMAP.md` com checklist de execução por fases (0 a 8).
- `README.md`, `CONTRIBUTING.md`, `SECURITY.md`.
- `AGENTS.md` com contexto e invariantes de segurança para agentes de IA.
- Configuração inicial do Cursor: rules (`.cursor/rules/`), hooks (`.cursor/hooks.json`) e skills (`.cursor/skills/`) específicas do projeto (auditoria de segurança do VPS, operações de peer WireGuard, checagem de registro de portas/domínios).
- Confirmação via DNS de que `vpn.officeempresa.com` e `ldpops.appapisip.com` apontam para `206.189.224.72`.
- `.gitignore` completo (segredos/chaves, artefatos de build, banco de dados local, arquivos de SO/IDE).
- Tabela de convenção de build (`PLAN.md` §11.1): o que é gerado, onde fica, e se é commitado.
- Hook real de pre-commit (`.githooks/pre-commit`), independente do editor, bloqueando commit de segredos e de artefatos de build — complementar (não substitui) o hook `.cursor/hooks.json`, que só protege ações do agente de IA dentro do Cursor.
- Repositório Git inicializado, com `core.hooksPath` configurado para `.githooks`.

### Fixed

- Identificada (ainda não corrigida no servidor) ambiguidade de configuração SSH (`PasswordAuthentication` divergente entre `50-cloud-init.conf` e `60-cloudimg-settings.conf`) — correção planejada na Fase 0 do `ROADMAP.md`.

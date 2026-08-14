# Changelog — xvpn-client

Changelog do componente `client`, mantido automaticamente pelo [release-please](https://github.com/googleapis/release-please) a partir da Fase 4. Ver [`../CHANGELOG.md`](../CHANGELOG.md) para mudanças "de projeto" que não pertencem a um componente específico.

## [0.1.0](https://github.com/rootkit-lab/xvpn/compare/client-v0.0.0...client-v0.1.0) — 2026-08-14

Primeira release empacotada do cliente desktop XVPN (Windows + Linux).

### Features

- **Redesign cyber azul** do cliente, área logada e painel web — dot-grid, cantos em L (cyber-frame), losango "Secured", labels HUD monoespaçados, arestas vivas. Mantém a paleta navy/azul/ciano. ([#25](https://github.com/rootkit-lab/xvpn/pull/25))
- **Recursos avançados do cliente**: kill switch, reconexão automática, split-tunnel, tray, autostart, diagnóstico. ([#9](https://github.com/rootkit-lab/xvpn/pull/9))
- **Redesign visual com tema glow azul**, animações e lazy-loading. ([#11](https://github.com/rootkit-lab/xvpn/pull/11))
- **Empacotamento .deb, AppImage e instalador NSIS** (Fase 7). ([#12](https://github.com/rootkit-lab/xvpn/pull/12))
- **Observabilidade**: logs estruturados e métricas. ([#13](https://github.com/rootkit-lab/xvpn/pull/13))
- **Compartilhamento de arquivos** via Samba e FileBrowser. ([#8](https://github.com/rootkit-lab/xvpn/pull/8))
- **Landing pública** com lista de espera e identidade visual. ([#10](https://github.com/rootkit-lab/xvpn/pull/10))

### Bug Fixes

- Fecha Fase 9: bugs de enroll/revoke, rate limit, cache, mutex, CI. ([#18](https://github.com/rootkit-lab/xvpn/pull/18))

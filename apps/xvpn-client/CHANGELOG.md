# Changelog — xvpn-client

Changelog do componente `client`, mantido automaticamente pelo [release-please](https://github.com/googleapis/release-please) a partir da Fase 4. Ver [`../CHANGELOG.md`](../CHANGELOG.md) para mudanças "de projeto" que não pertencem a um componente específico.

## [0.1.1](https://github.com/rootkit-lab/xvpn/compare/xvpn-client-v0.1.0...xvpn-client-v0.1.1) (2026-08-15)


### Features

* **client:** chrome watchOS, auth no Connect e estado verde ([#53](https://github.com/rootkit-lab/xvpn/issues/53)) ([a092d89](https://github.com/rootkit-lab/xvpn/commit/a092d898f975e05ea7406e1ace13eeb12c960bd9))
* redesign da home do cliente no estilo watchOS ([#52](https://github.com/rootkit-lab/xvpn/issues/52)) ([e6301dd](https://github.com/rootkit-lab/xvpn/commit/e6301ddbf0b18381241fda5e1061633a1cd181aa))
* **server:** alimenta o marketplace a partir de apps/ via CI (Fase 16) ([#37](https://github.com/rootkit-lab/xvpn/issues/37)) ([5ba7620](https://github.com/rootkit-lab/xvpn/commit/5ba762095e9c65b502694206387a2fab39e509de))
* sincroniza acesso a arquivos com o usuário da VPN (Fase 14) ([#35](https://github.com/rootkit-lab/xvpn/issues/35)) ([cf23b56](https://github.com/rootkit-lab/xvpn/commit/cf23b56b682f3a4d9cf3709907da4584b0e2e661))


### Bug Fixes

* abre Samba via ~/XVPN symlink + FileManager1 no Cosmic ([#48](https://github.com/rootkit-lab/xvpn/issues/48)) ([f711894](https://github.com/rootkit-lab/xvpn/commit/f7118947f909c21a81a71c22b4ac5738dccb991a))
* **client:** adiciona rota /32 de exceção no engine Windows ([#41](https://github.com/rootkit-lab/xvpn/issues/41)) ([4d0f3f3](https://github.com/rootkit-lab/xvpn/commit/4d0f3f39b96cf07dff28a3a465ea778306438f7f))
* desmonta shares SMB e limpa ~/XVPN/Desktop no Disconnect ([#49](https://github.com/rootkit-lab/xvpn/issues/49)) ([c9c6767](https://github.com/rootkit-lab/xvpn/commit/c9c676798c3e2db89b536538377913a9dd5db79f))
* desmonta shares SMB e limpa ~/XVPN/Desktop no Disconnect ([#50](https://github.com/rootkit-lab/xvpn/issues/50)) ([043b951](https://github.com/rootkit-lab/xvpn/commit/043b951f7ee7a3f703c809d9581336096dff6641))
* espera o path GVFS após gio mount --anonymous ([#47](https://github.com/rootkit-lab/xvpn/issues/47)) ([8e521fc](https://github.com/rootkit-lab/xvpn/commit/8e521fc035fba0c11e77e431906ce09782fb51be))
* monta Samba via GVFS anônimo antes de abrir no Cosmic ([#46](https://github.com/rootkit-lab/xvpn/issues/46)) ([c10ea53](https://github.com/rootkit-lab/xvpn/commit/c10ea53d63b680b7c7f35c7bf64831220a3272fc))
* UI do botão/globe, botões de arquivo e docs do FileBrowser ([#51](https://github.com/rootkit-lab/xvpn/issues/51)) ([839b7c7](https://github.com/rootkit-lab/xvpn/commit/839b7c71a76e9fa6a70bfb1aa33c36389ee6d0d1))

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

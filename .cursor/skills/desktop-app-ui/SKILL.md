---
name: desktop-app-ui
description: Design system dos apps desktop XVPN (xvpn-client, xvpn-chat e futuros). Use ao criar ou redesenhar UI em apps/xvpn-client, apps/xvpn-chat, WatchShell, watch-face, complications, tema dark, acento neon, ou quando o usuário pedir que um app siga o visual do cliente.
---

# UI dos apps desktop

Fonte da verdade: `apps/xvpn-client/frontend/src/index.css` + `components/watch-chrome.tsx`. **Copie** tokens e utilities — não importe CSS/componentes de um app no outro (builds Wails separados).

O painel web (`server/web`) **não** usa este sistema: continua navy/azul Workspace. Chat no chrome do painel é `inherit` (skill `chat-chrome`).

## Tokens (não invente outra paleta)

| Papel | Token | Valor |
|---|---|---|
| Fundo da janela | `--background` | `oklch(0.11 0.012 260)` (preto profundo) |
| Card / painel | `--card` | `oklch(0.18 0.014 260)` |
| Acento primário | `--primary` | azul `oklch(0.72 0.14 230)` |
| Ativo / meu / online | `--safe` / `--glow-safe` | verde neon `oklch(0.72–0.78 … 150)` |
| Tipo | `--font-display` | Outfit |

Utilities obrigatórias: `watch-face`, `watch-vignette`, `watch-complication`, `font-display`. Chrome: `WatchShell` / equivalente (`ChatShell` no chat). Botões de ícone circulares/`rounded-[10px]` com gradiente `from-white/16`. Cards `rounded-[18px]`–`[22px]`. Labels uppercase 10px tracking; títulos semibold Outfit.

## Regras

1. Default de um app desktop novo = este visual. Tema legado (ex. ICQ) só como opção, nunca como identidade.
2. Não copie o botão Power/anéis do client para outros apps — só a **linguagem** (tokens, vidro, tipografia, `--safe`).
3. Bolhas/ações do usuário: direita + `--safe`. Conteúdo do outro: esquerda + `watch-complication`.
4. Ações do composer/chamada (clipe, mic, telefone, vídeo): `ChatIconButton` filled (`rounded-[10px]`, ícone outline branco). Menus (status/tema) são dropdown ancorado, não faixa inline.
4. Ao mudar tokens no client, espelhe no chat (`index.css` + `theme/themes.scss`) na mesma PR se o chat quebrar visualmente.

## Onde não aplicar

Landing, `/my/login`, `/admin/login`, chrome Workspace, rail/popouts do chat (`inherit`).

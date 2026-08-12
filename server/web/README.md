# server/web — Painel XVPN

Painel administrativo em React + TypeScript + Tailwind v4 + shadcn/ui. Consumido pelo `xvpn-server` (`../`) via `go:embed` — ver [`../internal/webui/webui.go`](../internal/webui/webui.go).

Instruções de desenvolvimento (rodar backend + painel juntos, build, deploy) estão centralizadas em [`../README.md`](../README.md) para não duplicar documentação entre os dois lados do mesmo binário.

## Comandos

```bash
npm install       # instala dependências
npm run dev       # dev server com HMR (proxy /api -> 127.0.0.1:8080)
npm run build     # gera ../internal/webui/dist/ (embutido no binário Go)
npm run lint      # oxlint
```

## Convenções específicas deste diretório

- Componentes shadcn/ui em `src/components/ui/` — adicionados via `npx shadcn@latest add <componente>`. **Atenção**: em alguns ambientes o CLI não resolve o alias `@/` do `tsconfig` e cria os arquivos numa pasta literal `@/` na raiz — se isso acontecer, mova manualmente o conteúdo para `src/components/ui/` e remova a pasta `@/`.
- Cliente HTTP único em `src/lib/api.ts` — nunca `fetch` direto em componentes (ver `.cursor/rules/frontend-react.mdc`).
- `src/lib/auth-context.tsx` guarda o JWT em `localStorage`; um 401 de qualquer chamada limpa a sessão e redireciona para `/login`.

# Exemplos de package (Fase 45.3)

Fonte canónica no monorepo: [`server/internal/pkgexamples/fs/`](../../../internal/pkgexamples/fs/).
O boot do `xvpn-server` cria os slugs no XGIT e publica a versão `0.1.0`.

Cópia de trabalho local (não é o bare do VPS):

```sh
server/deploy/xgit/sync-package-examples.sh
# → $HOME/Projects/x/packages/{javascript,python,go,rust,generic}
```

| Pasta | Slug XGIT | Kind | Package |
|---|---|---|---|
| `javascript` | `hello-js` | npm | `@ihuull/hello-js` |
| `python` | `hello-py` | pypi | `hello-ihuull` |
| `go` (embed: `golang`) | `hello-go` | generic | `hello-go` |
| `rust` | `hello-rs` | generic | `hello-rs` |
| `generic` | `hello-bin` | generic | `hello-bin` |

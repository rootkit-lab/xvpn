# Exemplos de package (Fase 45.3)

Fonte canónica no monorepo: [`server/internal/pkgexamples/fs/`](../../../internal/pkgexamples/fs/).
O boot do `xvpn-server` cria os repos `xcorp/hello-*` no XGIT e publica a versão `0.1.0`.

Cópia de trabalho local (não é o bare do VPS):

```sh
server/deploy/xgit/sync-package-examples.sh
# → $HOME/Projects/x/packages/{javascript,python,go,rust,generic}
```

| Pasta | Repo XGIT | Kind | Package |
|---|---|---|---|
| `javascript` | `xcorp/hello-js` | npm | `@ihuull/hello-js` |
| `python` | `xcorp/hello-py` | pypi | `hello-ihuull` |
| `go` (embed: `golang`) | `xcorp/hello-go` | generic | `hello-go` |
| `rust` | `xcorp/hello-rs` | generic | `hello-rs` |
| `generic` | `xcorp/hello-bin` | generic | `hello-bin` |
| `maven` | `xcorp/hello-mvn` | maven | `com.ihuull:hello-mvn` |

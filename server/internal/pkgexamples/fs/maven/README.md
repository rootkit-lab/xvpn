# hello-mvn

Exemplo **Maven** no XGIT (`xcorp/hello-mvn`). Registry na malha:

```sh
git clone https://xgit.corp.ihuull.com/xcorp/hello-mvn
mvn deploy
```

Auth = JWE (Basic user + senha = token). Não grave o token no `pom.xml`.
`settings.xml` usa `${env.XVPN_PACKAGES_TOKEN}` no runner.

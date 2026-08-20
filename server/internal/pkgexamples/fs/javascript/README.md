# @ihuull/hello-js

Exemplo **npm** do registry XGIT. Clone e publique só na VPN (`xgit.corp`).
Path canónico: `xcorp/hello-js`.

```sh
git clone https://xgit.corp.ihuull.com/xcorp/hello-js
npm config set @ihuull:registry https://xgit.corp.ihuull.com/api/packages/xcorp/hello-js/npm/
npm install @ihuull/hello-js --registry https://xgit.corp.ihuull.com/api/packages/xcorp/hello-js/npm/
npm publish --registry https://xgit.corp.ihuull.com/api/packages/xcorp/hello-js/npm/
```

Auth: Bearer JWE (ou Basic com senha = JWE), igual ao git smart HTTP.
Não grave o token no `.npmrc` commitado.

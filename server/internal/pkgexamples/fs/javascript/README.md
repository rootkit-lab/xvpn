# @ihuull/hello-js

Exemplo **npm** do registry XGIT. Clone e publique só na VPN (`xgit.corp`).

```sh
npm config set @ihuull:registry https://xgit.corp.ihuull.com/api/packages/hello-js/npm/
npm install @ihuull/hello-js --registry https://xgit.corp.ihuull.com/api/packages/hello-js/npm/
```

Auth: Bearer JWE (ou Basic com senha = JWE), igual ao git smart HTTP.

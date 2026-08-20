# hello-ihuull

Exemplo **PyPI** do registry XGIT. Simple API (PEP 503 / PEP 691) em `xgit.corp`.

```sh
pip install hello-ihuull \
  --index-url https://<user>:<JWE>@xgit.corp.ihuull.com/api/packages/hello-py/pypi/simple/
```

Auth: Bearer ou Basic (senha = JWE). Sem hostname novo. Sem PyPI.org.

# hello-ihuull

Exemplo **PyPI** do registry XGIT. Simple API (PEP 503 / PEP 691) em `xgit.corp`.
Path canónico: `xcorp/hello-py`.

```sh
git clone https://xgit.corp.ihuull.com/xcorp/hello-py
pip install hello-ihuull \
  --index-url https://<user>:<JWE>@xgit.corp.ihuull.com/api/packages/xcorp/hello-py/pypi/simple/
twine upload --repository-url https://xgit.corp.ihuull.com/api/packages/xcorp/hello-py/pypi \
  -u <user> -p <JWE> dist/*
```

Auth: Bearer ou Basic (senha = JWE). Sem hostname novo. Sem PyPI.org.

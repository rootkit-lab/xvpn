---
name: python3
description: Use python3 no codespace para parse, JSON, testes e scripts. Sem bash; env via campo env, não VAR=valor.
---

# python3 no XCODESPACES

O terminal do agente **não é um shell**. `TESTE_WHO=Agente python3 …` é recusado (argv[0] vira `TESTE_WHO=Agente`).

## Como correr

```
run_terminal:
  argv: ["python3", "-c", "import os; print(os.environ.get('TESTE_WHO',''))"]
  env: { "TESTE_WHO": "Agente" }
  wait: true
```

- Sempre **espere** o comando (`wait` default, até 120s).
- Allowlist: `python3`, `git`, `go`, `npm`, `node`, `rg`. Sem bash/sudo/docker/ssh/curl.
- Stdlib só nesta fase (sem `pip install`).

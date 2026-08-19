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

Use `python3 -c` para one-liners. Espere o resultado (`wait` default). Sem bash. Stdlib apenas (sem pip).

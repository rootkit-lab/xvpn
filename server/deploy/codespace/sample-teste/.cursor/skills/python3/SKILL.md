---
name: python3
description: Use python3 no playground teste. env via campo env; espere o comando.
---

# python3

`run_terminal` argv `["python3", "-c", "..."]` + `env: { "TESTE_WHO": "Agente" }` + `wait: true`.
Não escreva `TESTE_WHO=Agente python3`.

Flask: `["python3", "web/flask/app.py"]` — o agente mostra a saída no terminal na hora (não trava em Waiting for shell).

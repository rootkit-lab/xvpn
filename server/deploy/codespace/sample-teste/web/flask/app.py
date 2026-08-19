#!/usr/bin/env python3
"""Canário Flask do XCODESPACES — escuta 0.0.0.0 para demo-<nome>.corp:8080."""

import os
import socket

from flask import Flask, jsonify

app = Flask(__name__)


@app.get("/")
def index():
    host = socket.gethostname()
    demo = os.environ.get("XCS_DEMO_HOST", "")
    return f"""<!DOCTYPE html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8" />
  <title>XCODESPACES demo Flask</title>
  <style>
    body {{ font-family: system-ui, sans-serif; background: #0a0a0f; color: #e8e8f0; margin: 2rem; }}
    h1 {{ color: #a78bfa; }}
    code {{ background: #1a1a24; padding: 0.2rem 0.4rem; border-radius: 4px; }}
  </style>
</head>
<body>
  <h1>demo Flask</h1>
  <p>Playground <strong>teste</strong> no container <code>{host}</code>.</p>
  <p>Preview intranet: <code>{demo or "demo-&lt;nome&gt;.corp.ihuull.com:8080"}</code></p>
  <p><a href="/health">/health</a></p>
</body>
</html>"""


@app.get("/health")
def health():
    return jsonify(ok=True, service="xcodespaces-demo-flask")


if __name__ == "__main__":
    port = int(os.environ.get("PORT", "8080"))
    app.run(host="0.0.0.0", port=port, debug=False)

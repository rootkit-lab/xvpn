#!/usr/bin/env python3
"""Valida apps/*/marketplace.yaml contra o schema mínimo da Fase 16."""
from __future__ import annotations

import re
import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    print("PyYAML é necessário: pip install pyyaml", file=sys.stderr)
    sys.exit(2)

ROOT = Path(__file__).resolve().parents[2]
APPS = ROOT / "apps"
REQUIRED = ("slug", "name", "description", "visibility", "source", "channel", "assets")
VISIBILITY = {"global", "restricted"}
SOURCES = {"build", "external"}
CHANNELS = {"stable", "beta"}
PLATFORMS = {"linux", "windows", "android"}


def fail(msg: str) -> None:
    print(f"ERRO: {msg}", file=sys.stderr)
    raise SystemExit(1)


def main() -> None:
    if not APPS.is_dir():
        print("apps/ ausente — nada a validar")
        return
    found = 0
    for path in sorted(APPS.glob("*/marketplace.yaml")):
        found += 1
        data = yaml.safe_load(path.read_text(encoding="utf-8"))
        if not isinstance(data, dict):
            fail(f"{path}: raiz deve ser um mapeamento")
        for key in REQUIRED:
            if key not in data:
                fail(f"{path}: campo obrigatório ausente: {key}")
        slug = data["slug"]
        if not re.fullmatch(r"[a-z0-9-]{2,20}", str(slug)):
            fail(f"{path}: slug {slug!r} inválido (use [a-z0-9-]{{2,20}})")
        # Disco pode diferir do slug (Fase 26: pasta apps/xvpn-chat, slug xchat).
        if data["visibility"] not in VISIBILITY:
            fail(f"{path}: visibility inválida")
        if data["source"] not in SOURCES:
            fail(f"{path}: source inválido")
        if data["channel"] not in CHANNELS:
            fail(f"{path}: channel inválido")
        assets = data["assets"]
        if not isinstance(assets, list) or not assets:
            fail(f"{path}: assets deve ser uma lista não vazia")
        for i, asset in enumerate(assets):
            if not isinstance(asset, dict):
                fail(f"{path}: assets[{i}] inválido")
            for k in ("platform", "arch", "url"):
                if k not in asset or not str(asset[k]).strip():
                    fail(f"{path}: assets[{i}] falta {k}")
            if asset["platform"] not in PLATFORMS:
                fail(f"{path}: assets[{i}] platform inválida")
            if data["source"] == "external":
                if not asset.get("sha256"):
                    fail(f"{path}: assets[{i}] source=external exige sha256")
                sha = str(asset["sha256"]).lower()
                if len(sha) != 64 or any(c not in "0123456789abcdef" for c in sha):
                    fail(f"{path}: assets[{i}] sha256 inválido")
            url = str(asset["url"])
            if not url.startswith("https://"):
                fail(f"{path}: assets[{i}] url deve ser https://")
        print(f"OK {path.relative_to(ROOT)}")
    if found == 0:
        print("Nenhum marketplace.yaml em apps/ — OK")
    else:
        print(f"{found} manifesto(s) válidos")


if __name__ == "__main__":
    main()

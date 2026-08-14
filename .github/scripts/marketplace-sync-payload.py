#!/usr/bin/env python3
"""Monta o JSON de POST /api/marketplace/sync a partir de apps/*/marketplace.yaml."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import sys
import urllib.error
import urllib.request
from pathlib import Path

try:
    import yaml
except ImportError:
    print("PyYAML é necessário: pip install pyyaml", file=sys.stderr)
    sys.exit(2)

ROOT = Path(__file__).resolve().parents[2]
MANIFEST = ROOT / ".release-please-manifest.json"


def load_versions() -> dict:
    if MANIFEST.is_file():
        return json.loads(MANIFEST.read_text(encoding="utf-8"))
    return {}


def sha256_url(url: str) -> str | None:
    try:
        with urllib.request.urlopen(url, timeout=60) as resp:
            h = hashlib.sha256()
            while True:
                chunk = resp.read(1024 * 1024)
                if not chunk:
                    break
                h.update(chunk)
            return h.hexdigest()
    except (urllib.error.URLError, urllib.error.HTTPError, TimeoutError) as exc:
        print(f"AVISO: não foi possível baixar {url}: {exc}", file=sys.stderr)
        return None


def resolve_url(template: str, version: str) -> str:
    return template.replace("${VERSION}", version)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--commit", default=os.environ.get("GITHUB_SHA", ""))
    parser.add_argument("--out", required=True)
    parser.add_argument("--skip-missing-build", action="store_true", default=True)
    args = parser.parse_args()

    versions = load_versions()
    apps_out: list[dict] = []
    apps_dir = ROOT / "apps"
    for path in sorted(apps_dir.glob("*/marketplace.yaml")):
        data = yaml.safe_load(path.read_text(encoding="utf-8"))
        slug = data["slug"]
        source = data["source"]
        channel = data.get("channel", "stable")
        version = data.get("version")
        if source == "build":
            version = versions.get(f"apps/{slug}") or versions.get(slug) or version
            if not version:
                print(f"AVISO: sem versão para {slug} — pulando", file=sys.stderr)
                continue
        if not version:
            print(f"AVISO: {path} sem version — pulando", file=sys.stderr)
            continue

        assets_out = []
        skip_app = False
        for asset in data.get("assets") or []:
            url = resolve_url(str(asset["url"]), str(version))
            sha = asset.get("sha256")
            if not sha:
                sha = sha256_url(url)
                if not sha:
                    if source == "build" and args.skip_missing_build:
                        print(f"AVISO: release ausente para {url} — pulando app {slug}", file=sys.stderr)
                        skip_app = True
                        break
                    print(f"ERRO: sha256 obrigatório para {url}", file=sys.stderr)
                    sys.exit(1)
            filename = asset.get("filename") or Path(url).name
            assets_out.append(
                {
                    "platform": asset["platform"],
                    "arch": asset.get("arch") or "amd64",
                    "url": url,
                    "sha256": str(sha).lower(),
                    "filename": filename,
                }
            )
        if skip_app:
            continue
        apps_out.append(
            {
                "slug": slug,
                "name": data["name"],
                "description": data.get("description") or "",
                "icon_url": data.get("icon_url") or "",
                "visibility": data.get("visibility") or "global",
                "source": source,
                "source_path": data.get("source_path") or f"apps/{slug}",
                "version": str(version),
                "channel": channel,
                "changelog": data.get("changelog") or "",
                "assets": assets_out,
            }
        )

    payload = {"commit_sha": args.commit, "apps": apps_out}
    Path(args.out).write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
    print(f"payload com {len(apps_out)} app(s) → {args.out}")


if __name__ == "__main__":
    main()

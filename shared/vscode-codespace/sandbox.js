"use strict";

const path = require("path");

const ALLOW_TERM = new Set(["git", "go", "npm", "npx", "node", "python3", "ls", "cat", "head", "rg"]);
const BLOCK_TERM = new Set(["docker", "sudo", "ssh", "scp", "curl", "wget", "nc", "nmap", "bash", "sh", "zsh"]);

function resolveWorkspacePath(root, rel) {
  if (!root) {
    throw new Error("workspace ausente");
  }
  const raw = String(rel || "").trim();
  if (!raw || raw.includes("\0")) {
    throw new Error("path inválido");
  }
  if (path.isAbsolute(raw)) {
    throw new Error("path absoluto recusado");
  }
  const rootAbs = path.resolve(root);
  const abs = path.resolve(rootAbs, raw);
  const relToRoot = path.relative(rootAbs, abs);
  if (relToRoot.startsWith("..") || path.isAbsolute(relToRoot)) {
    throw new Error("path fora do workspace");
  }
  return abs;
}

function allowTerminal(argv) {
  if (!Array.isArray(argv) || argv.length === 0) {
    return { ok: false, reason: "comando vazio" };
  }
  const bin = path.basename(String(argv[0]).trim().toLowerCase());
  if (!bin || BLOCK_TERM.has(bin) || !ALLOW_TERM.has(bin)) {
    return { ok: false, reason: "comando não permitido: " + bin };
  }
  const joined = argv.slice(1).join(" ");
  if (/\b(docker|sudo|ssh|curl|wget)\b/i.test(joined)) {
    return { ok: false, reason: "argv bloqueado" };
  }
  return { ok: true, bin };
}

module.exports = { resolveWorkspacePath, allowTerminal, ALLOW_TERM, BLOCK_TERM };

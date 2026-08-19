"use strict";

const fs = require("fs");
const path = require("path");

const ALLOW_TERM = new Set(["git", "go", "npm", "npx", "node", "python3", "ls", "cat", "head", "rg"]);
const BLOCK_TERM = new Set(["docker", "sudo", "ssh", "scp", "curl", "wget", "nc", "nmap", "bash", "sh", "zsh"]);

function posixRel(rel) {
  return String(rel || "").replace(/\\/g, "/");
}

function isDeniedRel(rel) {
  const n = posixRel(rel).replace(/^\.\//, "");
  if (n === ".git" || n.startsWith(".git/")) {
    return true;
  }
  if (n === "xvpn-credentials" || n.endsWith("/xvpn-credentials") || n.includes("/.git/")) {
    return true;
  }
  return false;
}

function underRoot(rootAbs, abs) {
  const rel = path.relative(rootAbs, abs);
  return rel !== "" && !rel.startsWith("..") && !path.isAbsolute(rel) ? rel : rel === "" ? "." : null;
}

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
  const lex = underRoot(rootAbs, abs);
  if (lex === null) {
    throw new Error("path fora do workspace");
  }
  if (isDeniedRel(lex)) {
    throw new Error("path bloqueado");
  }

  const realRoot = fs.existsSync(rootAbs) ? fs.realpathSync(rootAbs) : rootAbs;
  let candidate = abs;
  if (fs.existsSync(abs)) {
    candidate = fs.realpathSync(abs);
  } else {
    let parent = path.dirname(abs);
    while (!fs.existsSync(parent) && parent !== path.dirname(parent)) {
      parent = path.dirname(parent);
    }
    if (fs.existsSync(parent)) {
      candidate = path.join(fs.realpathSync(parent), path.relative(parent, abs));
    }
  }
  const real = underRoot(realRoot, candidate);
  if (real === null) {
    throw new Error("path fora do workspace");
  }
  if (isDeniedRel(real)) {
    throw new Error("path bloqueado");
  }
  return candidate;
}

function allowTerminal(argv) {
  if (!Array.isArray(argv) || argv.length === 0) {
    return { ok: false, reason: "comando vazio" };
  }
  const bin = path.basename(String(argv[0]).trim().toLowerCase());
  if (!bin || BLOCK_TERM.has(bin) || !ALLOW_TERM.has(bin)) {
    return { ok: false, reason: "comando não permitido: " + bin };
  }
  const rest = argv.slice(1).map(String);
  const joined = rest.join(" ");
  if (/\b(docker|sudo|ssh|curl|wget)\b/i.test(joined)) {
    return { ok: false, reason: "argv bloqueado" };
  }
  if (bin === "rg" && rest.some((a) => a === "--pre" || a.startsWith("--pre=") || a === "--pre-glob")) {
    return { ok: false, reason: "rg --pre bloqueado" };
  }
  if (bin === "git" && rest.some((a) => a === "--no-verify" || a === "--no-gpg-sign")) {
    return { ok: false, reason: "git --no-verify bloqueado" };
  }
  return { ok: true, bin };
}

module.exports = { resolveWorkspacePath, allowTerminal, isDeniedRel, ALLOW_TERM, BLOCK_TERM };

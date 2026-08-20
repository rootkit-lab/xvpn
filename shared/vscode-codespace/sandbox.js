"use strict";

const fs = require("fs");
const path = require("path");

const ALLOW_TERM = new Set([
  "git",
  "go",
  "gofmt",
  "npm",
  "npx",
  "node",
  "python3",
  "ls",
  "cat",
  "head",
  "rg",
  "xcs-analyze",
  "xcs-detect",
]);
const BLOCK_TERM = new Set(["docker", "sudo", "ssh", "scp", "curl", "wget", "nc", "nmap", "bash", "sh", "zsh"]);
const BLOCK_ENV = new Set([
  "PATH",
  "HOME",
  "SHELL",
  "LD_PRELOAD",
  "LD_LIBRARY_PATH",
  "LD_AUDIT",
  "PYTHONHOME",
  "PYTHONPATH",
  "NODE_OPTIONS",
  "NODE_PATH",
  "GIT_DIR",
  "GIT_WORK_TREE",
  "GIT_SSH",
  "GIT_SSH_COMMAND",
  "SSH_AUTH_SOCK",
  "SSH_COMMAND",
  "BASH_ENV",
  "ENV",
  "CDPATH",
  "NODE_TLS_REJECT_UNAUTHORIZED",
  "NODE_EXTRA_CA_CERTS",
  "GIT_SSL_NO_VERIFY",
  "GIT_SSL_CAINFO",
  "GIT_ALLOW_PROTOCOL",
  "PYTHONHTTPSVERIFY",
  "SSL_CERT_FILE",
  "SSL_CERT_DIR",
  "REQUESTS_CA_BUNDLE",
  "CURL_CA_BUNDLE",
  "NPM_CONFIG_STRICT_SSL",
  "GOPINSECURE",
  "PIP_TRUSTED_HOST",
]);
const BLOCK_ENV_RE = /TLS|SSL|VERIFY|INSECURE|PROXY|CERT|_CA_|PRELOAD|GIT_CONFIG/;
const ENV_KEY = /^[A-Z][A-Z0-9_]{0,63}$/;
const MAX_ENV_KEYS = 16;
const MAX_ENV_VAL = 256;

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

function sanitizeEnv(raw) {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    return {};
  }
  const out = {};
  for (const [k, v] of Object.entries(raw)) {
    const key = String(k);
    if (!ENV_KEY.test(key) || BLOCK_ENV.has(key) || BLOCK_ENV_RE.test(key)) {
      continue;
    }
    const val = String(v ?? "");
    if (val.length > MAX_ENV_VAL || /[\0\n\r]/.test(val)) {
      continue;
    }
    out[key] = val;
    if (Object.keys(out).length >= MAX_ENV_KEYS) {
      break;
    }
  }
  return out;
}

function mergeEnv(extra) {
  return { ...process.env, ...sanitizeEnv(extra) };
}

function allowTerminal(argv) {
  if (!Array.isArray(argv) || argv.length === 0) {
    return { ok: false, reason: "comando vazio" };
  }
  const raw0 = String(argv[0] || "").trim();
  if (raw0.includes("=")) {
    return {
      ok: false,
      reason: "não é shell — use env:{KEY:valor} e argv:['python3',...]",
    };
  }
  const bin = path.basename(raw0.toLowerCase());
  if (!bin || BLOCK_TERM.has(bin) || !ALLOW_TERM.has(bin)) {
    return { ok: false, reason: "comando não permitido: " + bin };
  }
  const rest = argv.slice(1).map(String);
  if ([raw0, ...rest].some((a) => /[\0\r\n]/.test(a))) {
    return { ok: false, reason: "argv com quebra de linha" };
  }
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

function echoLine(argv) {
  if (!Array.isArray(argv)) {
    return "";
  }
  return argv
    .map((a) => String(a).replace(/[\0\r\n]+/g, " ").replace(/\s+/g, " ").trim())
    .filter(Boolean)
    .join(" ")
    .slice(0, 200);
}

module.exports = {
  resolveWorkspacePath,
  allowTerminal,
  echoLine,
  sanitizeEnv,
  mergeEnv,
  isDeniedRel,
  ALLOW_TERM,
  BLOCK_TERM,
};

"use strict";

const fs = require("fs");
const path = require("path");
const { execFile } = require("child_process");
const { promisify } = require("util");
const { resolveWorkspacePath } = require("./sandbox");
const { listJobs } = require("./jobs");

const execFileAsync = promisify(execFile);
const FILE_CAP = 4 * 1024;
const SLASH_BUILTINS = [
  { id: "help", label: "/help — modos, @ # $ / e tools" },
  { id: "skills", label: "/skills — skills do repo" },
  { id: "commit", label: "/commit — generate commit" },
  { id: "explain", label: "/explain — explica o arquivo atual" },
];
const DOC_FILES = ["AGENTS.md", "README.md", "PLAN.md", "CONTRIBUTING.md", "ROADMAP.md", "SECURITY.md"];
const HASH_GIT = new Set(["git", "diff", "status", "log"]);
const HASH_DOCS = new Set(["docs", "doc"]);
const DOLLAR_TERM = new Set(["term", "jobs", "terminal", "shell"]);

function parseMentions(text) {
  const files = [];
  const hashes = [];
  const src = String(text || "");
  src.replace(/@([A-Za-z0-9._/-]+)/g, (_, p) => {
    files.push(p);
    return "";
  });
  src.replace(/#([A-Za-z0-9._/-]+)/g, (_, p) => {
    hashes.push(p.toLowerCase());
    return "";
  });
  const dollars = [];
  src.replace(/\$([A-Za-z0-9._/-]+)/g, (_, p) => {
    dollars.push(p.toLowerCase());
    return "";
  });
  const uniq = (xs) => [...new Set(xs)];
  const h = uniq(hashes);
  const d = uniq(dollars);
  return {
    files: uniq(files),
    hashes: h,
    dollars: d,
    git: h.some((x) => HASH_GIT.has(x)),
    docs: h.some((x) => HASH_DOCS.has(x) || DOC_FILES.some((d) => d.toLowerCase() === x)),
    term: d.some((x) => DOLLAR_TERM.has(x)),
    folders: h.filter((x) => !HASH_GIT.has(x) && !HASH_DOCS.has(x) && !DOC_FILES.some((d) => d.toLowerCase() === x)),
  };
}

function slashCommands(skillNames) {
  const extra = (skillNames || []).map((n) => ({ id: n, label: "/" + n + " — skill" }));
  return SLASH_BUILTINS.concat(extra);
}

function readCapped(root, rel) {
  try {
    const abs = resolveWorkspacePath(root, rel);
    const raw = fs.readFileSync(abs, "utf8");
    return raw.length > FILE_CAP ? raw.slice(0, FILE_CAP) + "\n…" : raw;
  } catch (_) {
    return "";
  }
}

async function mentionContext(root, text) {
  const m = parseMentions(text);
  const parts = [];
  for (const rel of m.files.slice(0, 8)) {
    const body = readCapped(root, rel);
    if (body) {
      parts.push("## @" + rel + "\n```\n" + body + "\n```");
    }
  }
  if (m.docs) {
    for (const name of DOC_FILES) {
      const body = readCapped(root, name);
      if (body) {
        parts.push("## #" + name + "\n" + body);
      }
    }
  }
  for (const rel of m.folders.slice(0, 6)) {
    try {
      const abs = resolveWorkspacePath(root, rel);
      const names = fs
        .readdirSync(abs, { withFileTypes: true })
        .filter((e) => e.name !== ".git")
        .slice(0, 40)
        .map((e) => (e.isDirectory() ? e.name + "/" : e.name));
      parts.push("## #" + rel + "\n" + names.join("\n"));
    } catch (_) {
      /* pasta inválida */
    }
  }
  if (m.git) {
    try {
      const { stdout } = await execFileAsync("git", ["status", "-sb"], { cwd: root, timeout: 5000 });
      const log = await execFileAsync("git", ["log", "-5", "--oneline"], { cwd: root, timeout: 5000 });
      parts.push("## #git\n" + String(stdout || "") + "\n" + String(log.stdout || ""));
    } catch (_) {
      parts.push("## #git\n(git indisponível)");
    }
  }
  if (m.term) {
    const recs = listJobs();
    if (!recs.length) {
      parts.push("## $term\n(nenhum job)");
    } else {
      parts.push(
        "## $term\n" +
          recs
            .map((j) => j.id + " " + j.status + (j.log ? " " + j.log : "") + "\n" + (j.out || ""))
            .join("\n---\n"),
      );
    }
  }
  return parts.join("\n\n").slice(0, 16 * 1024);
}

function listWorkspaceFiles(root) {
  const out = [];
  const walk = (rel, depth) => {
    if (out.length >= 80 || depth > 4) {
      return;
    }
    let abs;
    try {
      abs = resolveWorkspacePath(root, rel || ".");
    } catch (_) {
      return;
    }
    let ents = [];
    try {
      ents = fs.readdirSync(abs, { withFileTypes: true });
    } catch (_) {
      return;
    }
    for (const e of ents) {
      if (e.name === ".git" || e.name === "node_modules" || e.name === "vendor" || e.name === "dist") {
        continue;
      }
      const next = rel && rel !== "." ? path.posix.join(rel.replace(/\\/g, "/"), e.name) : e.name;
      if (e.isDirectory()) {
        walk(next, depth + 1);
      } else {
        out.push(next);
      }
    }
  };
  walk(".", 0);
  return out;
}

function listWorkspaceFolders(root) {
  try {
    return fs
      .readdirSync(resolveWorkspacePath(root, "."), { withFileTypes: true })
      .filter((e) => e.isDirectory() && e.name !== ".git" && e.name !== "node_modules")
      .slice(0, 24)
      .map((e) => e.name);
  } catch (_) {
    return [];
  }
}

function dollarChoices() {
  return [
    { id: "term", label: "$term — stdout dos jobs" },
    { id: "jobs", label: "$jobs — status dos background" },
  ];
}

function hashChoices(root) {
  const docs = DOC_FILES.filter((n) => {
    try {
      resolveWorkspacePath(root, n);
      return fs.existsSync(path.join(root, n));
    } catch (_) {
      return false;
    }
  }).map((n) => ({ id: n.toLowerCase(), label: "#" + n }));
  const folders = listWorkspaceFolders(root).map((n) => ({ id: n, label: "#" + n + "/" }));
  return [{ id: "git", label: "#git — status e log" }, { id: "docs", label: "#docs — README / AGENTS / PLAN" }].concat(
    docs,
    folders,
  );
}

module.exports = {
  parseMentions,
  slashCommands,
  mentionContext,
  listWorkspaceFiles,
  listWorkspaceFolders,
  hashChoices,
  dollarChoices,
  SLASH_BUILTINS,
  DOC_FILES,
};

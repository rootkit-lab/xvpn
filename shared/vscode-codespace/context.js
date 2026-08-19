"use strict";

const fs = require("fs");
const path = require("path");
const vscode = require("vscode");
const { resolveWorkspacePath } = require("./sandbox");

const AGENTS_CAP = 8 * 1024;
const RULE_CAP = 2 * 1024;
const FILE_CAP = 4 * 1024;

function readLimited(file, cap) {
  try {
    const raw = fs.readFileSync(file, "utf8");
    if (raw.length <= cap) {
      return raw;
    }
    return raw.slice(0, cap) + "\n…";
  } catch (_) {
    return "";
  }
}

function parseFrontmatter(raw) {
  const m = String(raw || "").match(/^---\n([\s\S]*?)\n---\n?([\s\S]*)$/);
  if (!m) {
    return { meta: {}, body: String(raw || "").trim() };
  }
  const meta = {};
  for (const line of m[1].split("\n")) {
    const kv = line.match(/^([A-Za-z0-9_-]+):\s*(.*)$/);
    if (kv) {
      meta[kv[1]] = kv[2].trim().replace(/^["']|["']$/g, "");
    }
  }
  return { meta, body: m[2].trim() };
}

function listSkills(root) {
  let dir;
  try {
    dir = resolveWorkspacePath(root, path.join(".cursor", "skills"));
  } catch (_) {
    return [];
  }
  let names = [];
  try {
    names = fs.readdirSync(dir);
  } catch (_) {
    return [];
  }
  const out = [];
  for (const name of names) {
    if (!/^[A-Za-z0-9._-]+$/.test(name)) {
      continue;
    }
    let file;
    try {
      file = resolveWorkspacePath(root, path.join(".cursor", "skills", name, "SKILL.md"));
    } catch (_) {
      continue;
    }
    const raw = readLimited(file, 32 * 1024);
    if (!raw) {
      continue;
    }
    const parsed = parseFrontmatter(raw);
    out.push({
      name: parsed.meta.name || name,
      description: parsed.meta.description || "",
      body: parsed.body,
      file,
    });
  }
  return out.sort((a, b) => a.name.localeCompare(b.name));
}

function listRules(root) {
  let dir;
  try {
    dir = resolveWorkspacePath(root, path.join(".cursor", "rules"));
  } catch (_) {
    return [];
  }
  let names = [];
  try {
    names = fs.readdirSync(dir);
  } catch (_) {
    return [];
  }
  const out = [];
  for (const name of names) {
    if (!name.endsWith(".mdc") && !name.endsWith(".md")) {
      continue;
    }
    if (!/^[A-Za-z0-9._-]+$/.test(name)) {
      continue;
    }
    let file;
    try {
      file = resolveWorkspacePath(root, path.join(".cursor", "rules", name));
    } catch (_) {
      continue;
    }
    const raw = readLimited(file, RULE_CAP);
    if (raw) {
      out.push({ name, text: raw });
    }
  }
  return out;
}

function currentFileSnippet() {
  const ed = vscode.window.activeTextEditor;
  if (!ed) {
    return "";
  }
  const sel = ed.document.getText(ed.selection);
  const folder = vscode.workspace.workspaceFolders?.[0];
  let rel = ed.document.uri.fsPath;
  if (folder) {
    rel = path.relative(folder.uri.fsPath, ed.document.uri.fsPath) || rel;
    try {
      resolveWorkspacePath(folder.uri.fsPath, rel);
    } catch (_) {
      return "";
    }
  }
  const text = (sel || ed.document.getText()).slice(0, FILE_CAP);
  if (!text.trim()) {
    return "";
  }
  return "Arquivo atual: " + rel + "\n```\n" + text + "\n```";
}

function buildContext(root, extraSkill) {
  const parts = [];
  let agents = "";
  try {
    agents = readLimited(resolveWorkspacePath(root, "AGENTS.md"), AGENTS_CAP);
  } catch (_) {
    agents = "";
  }
  if (agents) {
    parts.push("## AGENTS.md\n" + agents);
  }
  const skills = listSkills(root);
  if (skills.length) {
    parts.push(
      "## Skills\n" +
        skills.map((s) => "- " + s.name + ": " + s.description).join("\n") +
        "\nUse read_skill ou /nome para o corpo.",
    );
  }
  const rules = listRules(root);
  for (const r of rules.slice(0, 8)) {
    parts.push("## Rule " + r.name + "\n" + r.text);
  }
  const file = currentFileSnippet();
  if (file) {
    parts.push("## Editor\n" + file);
  }
  if (extraSkill) {
    const hit = skills.find((s) => s.name.toLowerCase() === extraSkill.toLowerCase());
    if (hit) {
      parts.push("## Skill " + hit.name + "\n" + hit.body.slice(0, 12 * 1024));
    }
  }
  return { text: parts.join("\n\n").slice(0, 24 * 1024), skills };
}

module.exports = { listSkills, listRules, buildContext, parseFrontmatter };

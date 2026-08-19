"use strict";

const fs = require("fs");
const path = require("path");
const { execFile } = require("child_process");
const { promisify } = require("util");
const { resolveWorkspacePath, allowTerminal } = require("./sandbox");
const { listSkills } = require("./context");
const { AGENT_TOOLS, READ_TOOLS, toolsForMode } = require("./tool-specs");

const execFileAsync = promisify(execFile);
const READ_CAP = 12 * 1024;
const OUT_CAP = 8 * 1024;

function parseArgs(raw) {
  if (!raw) {
    return {};
  }
  if (typeof raw === "object") {
    return raw;
  }
  try {
    return JSON.parse(raw);
  } catch (_) {
    return {};
  }
}

function needsConfirm(name) {
  return name === "write_file" || name === "apply_patch" || name === "run_terminal";
}

function confirmDetail(name, args) {
  if (name === "run_terminal") {
    return (args.argv || []).join(" ");
  }
  if (name === "write_file") {
    return "Escrever " + args.path + " (" + String(args.content || "").length + " bytes)";
  }
  if (name === "apply_patch") {
    return "Patch em " + args.path;
  }
  return name;
}

async function runTool(root, name, rawArgs) {
  const args = parseArgs(rawArgs);
  switch (name) {
    case "read_file": {
      const abs = resolveWorkspacePath(root, args.path);
      const buf = fs.readFileSync(abs);
      const text = buf.toString("utf8").slice(0, READ_CAP);
      return text + (buf.length > READ_CAP ? "\n…" : "");
    }
    case "list_dir": {
      const rel = args.path || ".";
      const abs = resolveWorkspacePath(root, rel);
      return fs
        .readdirSync(abs, { withFileTypes: true })
        .filter((e) => e.name !== ".git")
        .slice(0, 200)
        .map((e) => (e.isDirectory() ? e.name + "/" : e.name))
        .join("\n");
    }
    case "grep": {
      const pattern = String(args.pattern || "");
      if (!pattern) {
        throw new Error("pattern vazio");
      }
      const target = args.path ? resolveWorkspacePath(root, args.path) : resolveWorkspacePath(root, ".");
      try {
        const { stdout } = await execFileAsync(
          "rg",
          ["--no-config", "-n", "--max-count", "40", "-m", "40", "--", pattern, target],
          {
            cwd: root,
            env: { ...process.env, RIPGREP_CONFIG_PATH: "" },
            maxBuffer: OUT_CAP,
            timeout: 8000,
          },
        );
        return (stdout || "").slice(0, OUT_CAP) || "(sem matches)";
      } catch (err) {
        if (err && err.code === 1) {
          return "(sem matches)";
        }
        throw new Error(err instanceof Error ? err.message : "rg falhou");
      }
    }
    case "glob": {
      const pattern = String(args.pattern || "").trim();
      if (!pattern) {
        throw new Error("pattern vazio");
      }
      const target = args.path ? resolveWorkspacePath(root, args.path) : resolveWorkspacePath(root, ".");
      try {
        const { stdout } = await execFileAsync(
          "rg",
          ["--no-config", "--files", "-g", pattern, "--", target],
          {
            cwd: root,
            env: { ...process.env, RIPGREP_CONFIG_PATH: "" },
            maxBuffer: OUT_CAP,
            timeout: 8000,
          },
        );
        return (stdout || "").split("\n").filter(Boolean).slice(0, 200).join("\n") || "(sem matches)";
      } catch (err) {
        if (err && err.code === 1) {
          return "(sem matches)";
        }
        throw new Error(err instanceof Error ? err.message : "glob falhou");
      }
    }
    case "read_skill": {
      const hit = listSkills(root).find((s) => s.name.toLowerCase() === String(args.name || "").toLowerCase());
      if (!hit) {
        throw new Error("skill não encontrada");
      }
      return hit.body.slice(0, READ_CAP);
    }
    case "write_file": {
      const abs = resolveWorkspacePath(root, args.path);
      fs.mkdirSync(path.dirname(abs), { recursive: true });
      fs.writeFileSync(abs, String(args.content ?? ""), "utf8");
      return "escrito " + args.path;
    }
    case "apply_patch": {
      const abs = resolveWorkspacePath(root, args.path);
      const cur = fs.readFileSync(abs, "utf8");
      const oldS = String(args.old_string ?? "");
      if (!oldS || !cur.includes(oldS)) {
        throw new Error("trecho não encontrado");
      }
      fs.writeFileSync(abs, cur.replace(oldS, String(args.new_string ?? "")), "utf8");
      return "patch aplicado em " + args.path;
    }
    case "run_terminal": {
      const argv = Array.isArray(args.argv) ? args.argv.map(String) : [];
      const gate = allowTerminal(argv);
      if (!gate.ok) {
        throw new Error(gate.reason);
      }
      const { stdout, stderr } = await execFileAsync(argv[0], argv.slice(1), {
        cwd: root,
        maxBuffer: OUT_CAP,
        timeout: 20000,
      });
      return ((stdout || "") + (stderr ? "\n" + stderr : "")).slice(0, OUT_CAP) || "(ok)";
    }
    default:
      throw new Error("tool desconhecida: " + name);
  }
}

module.exports = { AGENT_TOOLS, READ_TOOLS, toolsForMode, needsConfirm, confirmDetail, runTool, parseArgs };

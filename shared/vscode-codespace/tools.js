"use strict";

const fs = require("fs");
const path = require("path");
const { execFile } = require("child_process");
const { promisify } = require("util");
const { resolveWorkspacePath, allowTerminal, mergeEnv, sanitizeEnv } = require("./sandbox");
const { startJob, snapshot, waitFor } = require("./jobs");
const { listSkills } = require("./context");
const { listMcp, callMcp, BAKED } = require("./mcp-host");
const { AGENT_TOOLS, READ_TOOLS, toolsForMode } = require("./tool-specs");

const execFileAsync = promisify(execFile);
const READ_CAP = 12 * 1024;
const OUT_CAP = 8 * 1024;
const TERM_WAIT_DEFAULT = 120000;
const TERM_WAIT_MAX = 180000;

function waitMs(raw) {
  const n = Number(raw);
  if (!Number.isFinite(n) || n <= 0) {
    return TERM_WAIT_DEFAULT;
  }
  return Math.min(Math.max(n, 1000), TERM_WAIT_MAX);
}

function formatJob(rec) {
  if (!rec) {
    return "job desapareceu";
  }
  const head = rec.id + " " + rec.status + (rec.code == null ? "" : " exit " + rec.code);
  const log = rec.log ? "\nlog " + rec.log : "";
  return (head + log + "\n" + (rec.out || "")).slice(0, OUT_CAP);
}

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

function needsConfirm(name, args) {
  if (name === "write_file" || name === "apply_patch" || name === "run_terminal") {
    return true;
  }
  if (name === "call_mcp") {
    const server = String((args && args.server) || "");
    return !BAKED.includes(server);
  }
  return false;
}

function confirmDetail(name, args) {
  if (name === "run_terminal") {
    const argv = (args.argv || []).join(" ");
    const keys = args.env && typeof args.env === "object" ? Object.keys(args.env) : [];
    return argv + (keys.length ? " env " + keys.join(",") : "");
  }
  if (name === "call_mcp") {
    return "MCP " + (args.server || "") + " " + (args.name || "") + " (python3 do clone)";
  }
  if (name === "write_file") {
    return "Escrever " + args.path + " (" + String(args.content || "").length + " bytes)";
  }
  if (name === "apply_patch") {
    return "Patch em " + args.path;
  }
  return name;
}

async function runTool(root, name, rawArgs, opts) {
  const args = parseArgs(rawArgs);
  const extRoot = opts && opts.extRoot ? opts.extRoot : __dirname;
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
      const hit = listSkills(root, extRoot).find((s) => s.name.toLowerCase() === String(args.name || "").toLowerCase());
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
    case "analyze_project": {
      try {
        const { stdout } = await execFileAsync("xcs-analyze", [root], {
          cwd: root,
          maxBuffer: OUT_CAP,
          timeout: 12000,
        });
        return (stdout || "").slice(0, OUT_CAP) || "(vazio)";
      } catch (err) {
        throw new Error(err instanceof Error ? err.message : "xcs-analyze falhou");
      }
    }
    case "job_status": {
      const rec = snapshot(String(args.id || ""));
      if (!rec) {
        throw new Error("job desconhecido");
      }
      return JSON.stringify(rec);
    }
    case "run_terminal": {
      const argv = Array.isArray(args.argv) ? args.argv.map(String) : [];
      const gate = allowTerminal(argv);
      if (!gate.ok) {
        throw new Error(gate.reason);
      }
      const extra = sanitizeEnv(args.env);
      const timeout = waitMs(args.wait_ms);
      const wait = args.wait !== false;
      if (args.background && !wait) {
        const rec = startJob(root, argv, undefined, extra);
        return "background " + rec.id + " " + argv.join(" ") + " (job_status depois)";
      }
      if (args.background && wait) {
        const rec = startJob(root, argv, undefined, extra);
        const done = await waitFor(rec.id, timeout);
        return formatJob(done);
      }
      const { stdout, stderr } = await execFileAsync(argv[0], argv.slice(1), {
        cwd: root,
        env: mergeEnv(extra),
        maxBuffer: OUT_CAP,
        timeout,
      });
      return ((stdout || "") + (stderr ? "\n" + stderr : "")).slice(0, OUT_CAP) || "(ok)";
    }
    case "list_mcp":
      return listMcp(root, extRoot);
    case "call_mcp":
      return callMcp(root, extRoot, args.server, args.name, args.arguments || args.args || {});
  }
}

module.exports = { AGENT_TOOLS, READ_TOOLS, toolsForMode, needsConfirm, confirmDetail, runTool, parseArgs };

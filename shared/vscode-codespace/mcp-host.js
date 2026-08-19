"use strict";

const fs = require("fs");
const path = require("path");
const { spawn } = require("child_process");
const { resolveWorkspacePath } = require("./sandbox");

const BAKED = ["think", "memory", "docs"];
const CALL_TIMEOUT = 20000;

function bakedScript(extRoot, name) {
  const root = fs.realpathSync(extRoot);
  const file = fs.realpathSync(path.join(root, "mcp", name + ".js"));
  if (!file.startsWith(root + path.sep) || path.dirname(file) !== path.join(root, "mcp")) {
    throw new Error("mcp bake inválido");
  }
  return file;
}

function loadCloneServers(workspace) {
  try {
    const raw = fs.readFileSync(path.join(workspace, ".cursor", "mcp.json"), "utf8");
    const data = JSON.parse(raw);
    const servers = data && data.mcpServers && typeof data.mcpServers === "object" ? data.mcpServers : {};
    const out = [];
    for (const [name, spec] of Object.entries(servers)) {
      if (!/^[a-z][a-z0-9_-]{0,32}$/.test(name)) {
        continue;
      }
      if (BAKED.includes(name)) {
        continue;
      }
      if (!spec || String(spec.command || "") !== "python3") {
        continue;
      }
      const args = Array.isArray(spec.args) ? spec.args.map(String) : [];
      if (args.length !== 1 || !args[0].startsWith(".cursor/mcp/") || !args[0].endsWith(".py")) {
        continue;
      }
      resolveWorkspacePath(workspace, args[0]);
      out.push({ name, command: "python3", args });
    }
    return out;
  } catch (_) {
    return [];
  }
}

function listServerSpecs(workspace, extRoot) {
  const baked = BAKED.map((name) => ({
    name,
    command: process.execPath,
    args: [bakedScript(extRoot, name)],
    baked: true,
  }));
  return baked.concat(loadCloneServers(workspace));
}

function rpcCall(spec, workspace, method, params) {
  return new Promise((resolve, reject) => {
    const child = spawn(spec.command, spec.args, {
      cwd: workspace || undefined,
      env: { ...process.env, XCS_WORKSPACE: workspace || "" },
      stdio: ["pipe", "pipe", "pipe"],
    });
    let buf = "";
    let settled = false;
    const timer = setTimeout(() => {
      child.kill("SIGTERM");
      finish(new Error("mcp timeout"));
    }, CALL_TIMEOUT);

    function finish(err, value) {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);
      if (err) {
        reject(err);
      } else {
        resolve(value);
      }
    }

    child.stdout.setEncoding("utf8");
    child.stdout.on("data", (chunk) => {
      buf += chunk;
      for (;;) {
        const n = buf.indexOf("\n");
        if (n < 0) {
          break;
        }
        const line = buf.slice(0, n).trim();
        buf = buf.slice(n + 1);
        if (!line) {
          continue;
        }
        let msg;
        try {
          msg = JSON.parse(line);
        } catch (_) {
          continue;
        }
        if (msg.id === 1 && msg.result) {
          child.stdin.write(
            JSON.stringify({ jsonrpc: "2.0", method: "notifications/initialized" }) + "\n",
          );
          child.stdin.write(JSON.stringify({ jsonrpc: "2.0", id: 2, method, params: params || {} }) + "\n");
        }
        if (msg.id === 2) {
          child.kill("SIGTERM");
          if (msg.error) {
            finish(new Error(msg.error.message || "mcp erro"));
          } else {
            finish(null, msg.result);
          }
        }
      }
    });
    child.on("error", (err) => finish(err));
    child.on("close", () => {
      if (!settled) {
        finish(new Error("mcp encerrou"));
      }
    });
    child.stdin.write(
      JSON.stringify({
        jsonrpc: "2.0",
        id: 1,
        method: "initialize",
        params: {
          protocolVersion: "2024-11-05",
          capabilities: {},
          clientInfo: { name: "ihuull.codespace", version: "0.4.0" },
        },
      }) + "\n",
    );
  });
}

function textOf(result) {
  const parts = result && Array.isArray(result.content) ? result.content : [];
  return parts
    .map((p) => (p && p.type === "text" ? String(p.text || "") : ""))
    .join("\n")
    .trim();
}

async function listMcp(workspace, extRoot) {
  const specs = listServerSpecs(workspace, extRoot);
  const out = [];
  for (const spec of specs) {
    if (!spec.baked) {
      out.push({
        server: spec.name,
        baked: false,
        tools: [],
        note: "python3 do clone — call_mcp pede Aplicar",
      });
      continue;
    }
    try {
      const listed = await rpcCall(spec, workspace, "tools/list", {});
      const tools = Array.isArray(listed.tools) ? listed.tools : [];
      out.push({
        server: spec.name,
        baked: Boolean(spec.baked),
        tools: tools.map((t) => ({ name: t.name, description: t.description || "" })),
      });
    } catch (err) {
      out.push({ server: spec.name, baked: Boolean(spec.baked), error: err instanceof Error ? err.message : "falhou" });
    }
  }
  return JSON.stringify(out, null, 2);
}

async function callMcp(workspace, extRoot, server, name, args) {
  const spec = listServerSpecs(workspace, extRoot).find((s) => s.name === String(server || ""));
  if (!spec) {
    throw new Error("mcp desconhecido: " + server);
  }
  const result = await rpcCall(spec, workspace, "tools/call", {
    name: String(name || ""),
    arguments: args && typeof args === "object" ? args : {},
  });
  return textOf(result) || JSON.stringify(result);
}

module.exports = { listServerSpecs, listMcp, callMcp, BAKED };

"use strict";

const fs = require("fs");

function serve(meta, tools) {
  const map = new Map((tools || []).map((t) => [t.name, t]));
  let buf = "";
  process.stdin.setEncoding("utf8");
  process.stdin.on("data", (chunk) => {
    buf += String(chunk);
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
      handle(msg).catch(() => {});
    }
  });

  function send(obj) {
    fs.writeSync(1, JSON.stringify(obj) + "\n");
  }

  async function handle(msg) {
    if (!msg || typeof msg !== "object") {
      return;
    }
    const method = String(msg.method || "");
    if (method === "initialize") {
      send({
        jsonrpc: "2.0",
        id: msg.id,
        result: {
          protocolVersion: "2024-11-05",
          capabilities: { tools: {} },
          serverInfo: { name: meta.name || "xcs-mcp", version: meta.version || "0.1.0" },
        },
      });
      return;
    }
    if (method === "notifications/initialized" || method === "initialized") {
      return;
    }
    if (method === "tools/list") {
      send({
        jsonrpc: "2.0",
        id: msg.id,
        result: {
          tools: (tools || []).map((t) => ({
            name: t.name,
            description: t.description || "",
            inputSchema: t.inputSchema || { type: "object", properties: {} },
          })),
        },
      });
      return;
    }
    if (method === "tools/call") {
      const name = String((msg.params && msg.params.name) || "");
      const t = map.get(name);
      if (!t) {
        send({ jsonrpc: "2.0", id: msg.id, error: { code: -32601, message: "tool desconhecida" } });
        return;
      }
      try {
        const text = await t.handler((msg.params && msg.params.arguments) || {});
        send({
          jsonrpc: "2.0",
          id: msg.id,
          result: { content: [{ type: "text", text: String(text || "") }] },
        });
      } catch (err) {
        send({
          jsonrpc: "2.0",
          id: msg.id,
          error: { code: -32000, message: err instanceof Error ? err.message : "falhou" },
        });
      }
    }
  }
}

module.exports = { serve };

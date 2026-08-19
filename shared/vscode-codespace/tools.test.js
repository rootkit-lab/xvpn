"use strict";

const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const assert = require("node:assert/strict");
const { toolsForMode, AGENT_TOOLS, READ_TOOLS } = require("./tool-specs");
const { toolCardTitle, exploreLabel } = require("./chat-ui");

test("toolsForMode ask não manda tools", () => {
  assert.deepEqual(toolsForMode("ask"), []);
});

test("toolsForMode plan só lê", () => {
  const names = toolsForMode("plan").map((t) => t.function.name);
  assert.deepEqual(names, READ_TOOLS.map((t) => t.function.name));
  assert.ok(names.includes("glob"));
  assert.ok(!names.includes("write_file"));
  assert.ok(!names.includes("run_terminal"));
});

test("toolsForMode agent e debug usam o set completo", () => {
  assert.equal(toolsForMode("agent").length, AGENT_TOOLS.length);
  assert.equal(toolsForMode("debug").length, AGENT_TOOLS.length);
});

test("loop do agente sobe o teto e resume em vez de só pedir reformular", () => {
  const src = fs.readFileSync(path.join(__dirname, "extension.js"), "utf8");
  assert.match(src, /const MAX_AGENT_TURNS = 24/);
  assert.match(src, /const MAX_LLM_MSGS = 76/);
  assert.match(src, /Teto de tools\. Responda com o que já descobriu/);
  assert.match(src, /finishCeiling/);
  assert.doesNotMatch(src, /reformule o pedido/);
  assert.doesNotMatch(src, /tool " \+ i/);
});

test("toolCardTitle e exploreLabel no estilo Cursor", () => {
  assert.equal(toolCardTitle("read_file", { path: "web/index.mjs" }), "Leu index.mjs");
  assert.equal(toolCardTitle("grep", { pattern: "greeting" }), "Buscou greeting");
  assert.equal(toolCardTitle("run_terminal", { argv: ["go", "test", "./..."] }), "Rodou go test ./...");
  assert.equal(exploreLabel(["read_file", "read_file", "grep"]), "Explorou 2 arquivos, 1 busca");
});

test("webview do chat tem timeline Thought / cards / composer", () => {
  const html = fs.readFileSync(path.join(__dirname, "agent.html"), "utf8");
  assert.match(html, /className = 'thought live'/);
  assert.match(html, /\.tool-card/);
  assert.match(html, /Explorou/);
  assert.match(html, /aria-label="Enviar"/);
  assert.doesNotMatch(html, /tool 6/);
});

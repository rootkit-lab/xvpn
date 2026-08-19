"use strict";

const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");
const assert = require("node:assert/strict");
const { toolsForMode, AGENT_TOOLS, READ_TOOLS } = require("./tool-specs");
const { toolCardTitle, exploreLabel } = require("./chat-ui");
const { parseMentions, slashCommands } = require("./mentions");

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
  assert.equal(toolCardTitle("run_terminal", { argv: ["go", "test", "./..."] }), "Aguardou go test ./...");
  assert.equal(exploreLabel(["read_file", "read_file", "grep"]), "Explorou 2 arquivos, 1 busca");
});

test("parseMentions lê @arquivo #git $term e /", () => {
  const m = parseMentions("olha @go.mod e #git $term depois /explain");
  assert.deepEqual(m.files, ["go.mod"]);
  assert.equal(m.git, true);
  assert.equal(m.term, true);
  assert.ok(slashCommands([]).some((c) => c.id === "explain"));
});

test("toolsForMode plan inclui analyze_project e não terminal", () => {
  const names = toolsForMode("plan").map((t) => t.function.name);
  assert.ok(names.includes("analyze_project"));
  assert.ok(!names.includes("run_terminal"));
});

test("webview do chat tem timeline Thought / cards / composer", () => {
  const html = fs.readFileSync(path.join(__dirname, "agent.html"), "utf8");
  assert.match(html, /className = 'thought live'/);
  assert.match(html, /\.tool-card/);
  assert.match(html, /Explorou/);
  assert.match(html, /aria-label="Enviar"/);
  assert.match(html, /@arquivo/);
  assert.match(html, /id="palette"/);
  assert.match(html, /id="review"/);
  assert.match(html, /id="stop"/);
  assert.match(html, /data-insert="\$"/);
  assert.doesNotMatch(html, /tool 6/);
});

test("writeArtifact grava em .cursor/agent e fileDelta conta linhas", () => {
  const os = require("node:os");
  const { writeArtifact, fileDelta, shouldDump } = require("./artifacts");
  const { readHooks } = require("./hooks");
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "xcs-art-"));
  try {
    assert.equal(shouldDump("read_file", "curto"), false);
    assert.equal(shouldDump("run_terminal", "ok"), true);
    const dump = writeArtifact(root, "run_terminal", "Rodou ls", "a".repeat(80));
    assert.match(dump.path, /^\.cursor\/agent\//);
    assert.ok(fs.existsSync(path.join(root, dump.path)));
    assert.ok(dump.preview.length <= 240);
    const delta = fileDelta("write_file", { path: "a.go", content: "1\n2\n3\n" }, "old\n");
    assert.equal(delta.path, "a.go");
    assert.equal(delta.add, 4);
    assert.equal(delta.del, 2);
    fs.mkdirSync(path.join(root, ".cursor"), { recursive: true });
    fs.writeFileSync(
      path.join(root, ".cursor", "hooks.json"),
      JSON.stringify({ version: 1, hooks: { beforeShellExecution: [], afterFileEdit: [] } }),
    );
    assert.deepEqual(readHooks(root).events.sort(), ["afterFileEdit", "beforeShellExecution"]);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

test("loop do agente tem Stop, Review, artifact, wait e MCP", () => {
  const src = fs.readFileSync(path.join(__dirname, "extension.js"), "utf8");
  assert.match(src, /type === "stop"/);
  assert.match(src, /writeArtifact/);
  assert.match(src, /fileDelta/);
  assert.match(src, /phase: "editing"/);
  assert.match(src, /parsed.background && parsed.wait === false/);
  assert.match(src, /echoLine/);
  assert.match(src, /this.abort.signal/);
  assert.match(src, /list_mcp/);
});

test("listSkills recusa SKILL.md symlink para .git", () => {
  const { listSkills } = require("./context");
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "xcs-sk-"));
  try {
    fs.mkdirSync(path.join(tmp, ".git"), { recursive: true });
    fs.writeFileSync(path.join(tmp, ".git", "xvpn-credentials"), "secret-token");
    fs.mkdirSync(path.join(tmp, ".cursor", "skills", "evil"), { recursive: true });
    fs.symlinkSync(path.join(tmp, ".git", "xvpn-credentials"), path.join(tmp, ".cursor", "skills", "evil", "SKILL.md"));
    assert.equal(listSkills(tmp).length, 0);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test("call_mcp do clone pede confirmação; think bakeado não", () => {
  const { needsConfirm } = require("./tools");
  assert.equal(needsConfirm("call_mcp", { server: "think", name: "think" }), false);
  assert.equal(needsConfirm("call_mcp", { server: "custom", name: "foo" }), true);
  assert.equal(needsConfirm("run_terminal", { argv: ["python3"] }), true);
});

test("MCP think responde e python3 espera env", async () => {
  const { callMcp, listMcp } = require("./mcp-host");
  const { runTool } = require("./tools");
  const listed = JSON.parse(await listMcp(__dirname, __dirname));
  assert.ok(listed.some((s) => s.server === "think" && s.tools.some((t) => t.name === "think")));
  const thought = await callMcp(__dirname, __dirname, "think", "think", { thought: "usar python3" });
  assert.match(thought, /python3/);
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "xcs-py-"));
  try {
    const out = await runTool(
      root,
      "run_terminal",
      {
        argv: ["python3", "-c", "import os; print(os.environ.get('TESTE_WHO','missing'))"],
        env: { TESTE_WHO: "Agente" },
        wait: true,
      },
      { extRoot: __dirname },
    );
    assert.match(String(out), /Agente/);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

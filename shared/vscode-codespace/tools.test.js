"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { toolsForMode, AGENT_TOOLS, READ_TOOLS } = require("./tool-specs");

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

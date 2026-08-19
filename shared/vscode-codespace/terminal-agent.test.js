"use strict";

const { test } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const path = require("path");
const {
  sendTerminalHereDoc,
  isBackgroundFireAndForget,
} = require("./terminal-agent");

test("isBackgroundFireAndForget", () => {
  assert.equal(isBackgroundFireAndForget({ background: true, wait: false }), true);
  assert.equal(isBackgroundFireAndForget({ background: true, wait: true }), false);
  assert.equal(isBackgroundFireAndForget({ background: false }), false);
});

test("sendTerminalHereDoc preserva quebras de linha", () => {
  const sent = [];
  const term = { sendText(line, execute) {
    sent.push({ line, execute });
  } };
  sendTerminalHereDoc(term, "linha1\nlinha2\n");
  assert.equal(sent.length, 1);
  assert.equal(sent[0].execute, true);
  assert.match(sent[0].line, /^cat <<'XCS_/);
  assert.match(sent[0].line, /linha1\nlinha2/);
});

test("extension não usa prefixo # agent:", () => {
  const src = fs.readFileSync(path.join(__dirname, "extension.js"), "utf8");
  assert.doesNotMatch(src, /# agent:/);
  assert.match(src, /prepareAgentTerminal/);
  assert.match(src, /finishAgentTerminal/);
});

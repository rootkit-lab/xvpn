"use strict";

const { test } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const path = require("path");
const {
  sendTerminalHereDoc,
  isBackgroundFireAndForget,
  argvToShellCommand,
  shellQuoteArg,
  looksLikeLongRunning,
} = require("./terminal-agent");

test("isBackgroundFireAndForget", () => {
  assert.equal(isBackgroundFireAndForget({ background: true, wait: false }), true);
  assert.equal(isBackgroundFireAndForget({ background: true, wait: true }), false);
  assert.equal(isBackgroundFireAndForget({ background: false }), false);
});

test("argvToShellCommand faz quoting de metacaracteres", () => {
  assert.equal(argvToShellCommand(["python3", "-c", "print(1); print(2)"]), "python3 -c 'print(1); print(2)'");
  assert.equal(argvToShellCommand(["go", "run", "./cmd/hello"]), "go run ./cmd/hello");
  assert.equal(shellQuoteArg("a'b"), "'a'\\''b'");
});

test("looksLikeLongRunning detecta Flask e app.py", () => {
  assert.equal(looksLikeLongRunning(["python3", "web/flask/app.py"]), true);
  assert.equal(looksLikeLongRunning(["python3", "-m", "flask", "run", "--host", "0.0.0.0"]), true);
  assert.equal(looksLikeLongRunning(["python3", "-c", "print(1)"]), false);
  assert.equal(looksLikeLongRunning(["go", "test", "./..."]), false);
});

test("sendTerminalHereDoc preserva quebras de linha", () => {
  const sent = [];
  const term = {
    sendText(line, execute) {
      sent.push({ line, execute });
    },
  };
  sendTerminalHereDoc(term, "linha1\nlinha2\n");
  assert.equal(sent.length, 1);
  assert.equal(sent[0].execute, true);
  assert.match(sent[0].line, /^cat <<'XCS_/);
  assert.match(sent[0].line, /linha1\nlinha2/);
});

test("extension usa PTY ao vivo e não # agent:", () => {
  const src = fs.readFileSync(path.join(__dirname, "extension.js"), "utf8");
  assert.doesNotMatch(src, /# agent:/);
  assert.match(src, /attachAgentTerminal/);
  assert.match(src, /onChunk/);
});

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

test("sendTerminalHereDoc remove \\r da saída espelhada", () => {
  const sent = [];
  const term = {
    sendText(line, execute) {
      sent.push({ line, execute });
    },
  };
  sendTerminalHereDoc(term, "ok\r; curl evil\n");
  assert.equal(sent.length, 1);
  assert.doesNotMatch(sent[0].line, /\r/);
  assert.match(sent[0].line, /ok; curl evil/);
});

test("extension não usa prefixo # agent:", () => {
  const src = fs.readFileSync(path.join(__dirname, "extension.js"), "utf8");
  assert.doesNotMatch(src, /# agent:/);
  assert.match(src, /finishAgentTerminal/);
});

test("terminal-agent não executa argv cru no PTY", () => {
  const src = fs.readFileSync(path.join(__dirname, "terminal-agent.js"), "utf8");
  assert.doesNotMatch(src, /sendText\(line/);
  assert.match(src, /sendTerminalHereDoc/);
});

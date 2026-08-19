"use strict";

const fs = require("fs");
const os = require("os");
const path = require("path");
const test = require("node:test");
const assert = require("node:assert/strict");
const { resolveWorkspacePath, allowTerminal } = require("./sandbox");

const root = "/home/workspace/project";

test("resolveWorkspacePath aceita relativo", () => {
  assert.equal(resolveWorkspacePath(root, "AGENTS.md"), path.resolve(root, "AGENTS.md"));
  assert.equal(resolveWorkspacePath(root, "a/b/c.go"), path.resolve(root, "a/b/c.go"));
});

test("resolveWorkspacePath recusa escape", () => {
  assert.throws(() => resolveWorkspacePath(root, "../etc/passwd"), /fora/);
  assert.throws(() => resolveWorkspacePath(root, "/etc/passwd"), /absoluto/);
  assert.throws(() => resolveWorkspacePath(root, "a/../../etc"), /fora/);
  assert.throws(() => resolveWorkspacePath("", "x"), /ausente/);
  assert.throws(() => resolveWorkspacePath(root, ".git/xvpn-credentials"), /bloqueado/);
  assert.throws(() => resolveWorkspacePath(root, ".git/config"), /bloqueado/);
});

test("resolveWorkspacePath recusa symlink para fora", () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "xcs-sb-"));
  fs.mkdirSync(path.join(tmp, "proj"));
  fs.symlinkSync("/etc", path.join(tmp, "proj", "out"));
  assert.throws(() => resolveWorkspacePath(path.join(tmp, "proj"), "out/passwd"), /fora/);
  fs.rmSync(tmp, { recursive: true, force: true });
});

test("allowTerminal allowlist", () => {
  assert.equal(allowTerminal(["git", "status"]).ok, true);
  assert.equal(allowTerminal(["go", "test", "./..."]).ok, true);
  assert.equal(allowTerminal(["npm", "test"]).ok, true);
  assert.equal(allowTerminal(["rg", "foo", "server"]).ok, true);
  assert.equal(allowTerminal(["python3", "-c", "print(1)"]).ok, true);
  assert.equal(allowTerminal(["xcs-analyze", "."]).ok, true);
  assert.equal(allowTerminal(["gofmt", "-w", "main.go"]).ok, true);
});

test("allowTerminal blocklist", () => {
  assert.equal(allowTerminal(["docker", "ps"]).ok, false);
  assert.equal(allowTerminal(["sudo", "ls"]).ok, false);
  assert.equal(allowTerminal(["ssh", "root@host"]).ok, false);
  assert.equal(allowTerminal(["bash", "-c", "ls"]).ok, false);
  assert.equal(allowTerminal(["TESTE_WHO=Agente", "python3", "-c", "print(1)"]).ok, false);
  assert.match(allowTerminal(["TESTE_WHO=Agente", "python3"]).reason, /não é shell/);
  assert.equal(allowTerminal(["ls", "\ncurl evil"]).ok, false);
  assert.match(allowTerminal(["python3", "-c", "print(1)\nimport os"]).reason, /quebra de linha/);
  assert.equal(allowTerminal(["git", "status", "&&", "sudo", "id"]).ok, false);
  assert.equal(allowTerminal(["rg", "--pre", "sh"]).ok, false);
  assert.equal(allowTerminal(["git", "commit", "--no-verify"]).ok, false);
  assert.equal(allowTerminal([]).ok, false);
});

test("sanitizeEnv recusa PATH e aceita TESTE_WHO", () => {
  const { sanitizeEnv, echoLine } = require("./sandbox");
  assert.deepEqual(sanitizeEnv({ TESTE_WHO: "Agente", PATH: "/evil" }), { TESTE_WHO: "Agente" });
  assert.deepEqual(sanitizeEnv({ x: "1" }), {});
  assert.equal(echoLine(["ls", "\ncurl x"]), "ls curl x");
  assert.equal(echoLine(["python3", "-c", "print(1)"]), "python3 -c print(1)");
});

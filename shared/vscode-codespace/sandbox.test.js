"use strict";

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
});

test("allowTerminal allowlist", () => {
  assert.equal(allowTerminal(["git", "status"]).ok, true);
  assert.equal(allowTerminal(["go", "test", "./..."]).ok, true);
  assert.equal(allowTerminal(["npm", "test"]).ok, true);
  assert.equal(allowTerminal(["rg", "foo", "server"]).ok, true);
});

test("allowTerminal blocklist", () => {
  assert.equal(allowTerminal(["docker", "ps"]).ok, false);
  assert.equal(allowTerminal(["sudo", "ls"]).ok, false);
  assert.equal(allowTerminal(["ssh", "root@host"]).ok, false);
  assert.equal(allowTerminal(["bash", "-c", "ls"]).ok, false);
  assert.equal(allowTerminal(["git", "status", "&&", "sudo", "id"]).ok, false);
  assert.equal(allowTerminal([]).ok, false);
});

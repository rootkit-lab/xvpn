"use strict";

const { spawn } = require("child_process");
const { allowTerminal } = require("./sandbox");

const OUT_CAP = 8 * 1024;
const MAX_RUNNING = 3;

let seq = 0;
const jobs = new Map();

function startJob(root, argv, onUpdate) {
  const gate = allowTerminal(argv);
  if (!gate.ok) {
    throw new Error(gate.reason);
  }
  if (runningCount() >= MAX_RUNNING) {
    throw new Error("teto de terminais em background");
  }
  const id = "j" + ++seq;
  const child = spawn(argv[0], argv.slice(1), { cwd: root, env: process.env });
  const rec = { id, argv, status: "running", out: "", code: null, child };
  jobs.set(id, rec);
  const push = (chunk) => {
    rec.out = (rec.out + chunk).slice(-OUT_CAP);
    if (onUpdate) {
      onUpdate(snapshot(id));
    }
  };
  child.stdout.on("data", (b) => push(b.toString("utf8")));
  child.stderr.on("data", (b) => push(b.toString("utf8")));
  child.on("error", (err) => {
    rec.status = "fail";
    rec.out = (rec.out + "\n" + err.message).slice(-OUT_CAP);
    if (onUpdate) {
      onUpdate(snapshot(id));
    }
  });
  child.on("close", (code) => {
    rec.status = code === 0 ? "ok" : "fail";
    rec.code = code;
    rec.child = null;
    if (onUpdate) {
      onUpdate(snapshot(id));
    }
  });
  return snapshot(id);
}

function snapshot(id) {
  const rec = jobs.get(id);
  if (!rec) {
    return null;
  }
  return {
    id: rec.id,
    status: rec.status,
    argv: rec.argv,
    out: rec.out.slice(-2000),
    code: rec.code,
  };
}

function runningCount() {
  let n = 0;
  for (const rec of jobs.values()) {
    if (rec.status === "running") {
      n += 1;
    }
  }
  return n;
}

module.exports = { startJob, snapshot, runningCount };

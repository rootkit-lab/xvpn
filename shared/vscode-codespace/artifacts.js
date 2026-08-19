"use strict";

const fs = require("fs");
const path = require("path");

const BODY_CAP = 32 * 1024;
const PREVIEW_CAP = 240;
const DUMP_KINDS = new Set(["run_terminal", "grep", "analyze_project", "job_status"]);

function artifactDir(root) {
  const cursor = path.join(root || "", ".cursor", "agent");
  try {
    fs.mkdirSync(cursor, { recursive: true });
    return { dir: cursor, rel: Boolean(root) };
  } catch (_) {
    const tmp = "/tmp/xcs-agent";
    fs.mkdirSync(tmp, { recursive: true });
    return { dir: tmp, rel: false };
  }
}

function previewOf(body) {
  return String(body || "").slice(0, PREVIEW_CAP);
}

function shouldDump(kind, body) {
  if (DUMP_KINDS.has(String(kind || ""))) {
    return true;
  }
  return String(body || "").length > 400;
}

function writeArtifact(root, kind, title, body) {
  const text = String(body || "");
  const preview = previewOf(text);
  if (!shouldDump(kind, text)) {
    return { path: "", preview: text.slice(0, 320) };
  }
  const { dir, rel } = artifactDir(root);
  const id = Date.now().toString(16) + "-" + String(kind || "tool").replace(/[^a-z0-9_-]/gi, "");
  const file = path.join(dir, id + ".log");
  fs.writeFileSync(file, "# " + String(title || kind) + "\n\n" + text.slice(0, BODY_CAP), "utf8");
  if (rel && root) {
    return { path: path.relative(root, file).replace(/\\/g, "/"), preview };
  }
  return { path: file, preview };
}

function fileDelta(name, args, before) {
  const a = args && typeof args === "object" ? args : {};
  if (name === "write_file" && a.path) {
    const after = String(a.content || "");
    return {
      path: String(a.path),
      add: after.split("\n").length,
      del: before ? String(before).split("\n").length : 0,
    };
  }
  if (name === "apply_patch" && a.path) {
    return {
      path: String(a.path),
      add: String(a.new_string || "").split("\n").length,
      del: String(a.old_string || "").split("\n").length,
    };
  }
  return null;
}

module.exports = { artifactDir, writeArtifact, fileDelta, shouldDump, BODY_CAP, PREVIEW_CAP };

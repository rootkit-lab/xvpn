"use strict";

const { execFile } = require("child_process");
const { promisify } = require("util");

const execFileAsync = promisify(execFile);

// openvscode-server escuta 3000 no container — não é preview do projeto.
const SKIP = new Set([3000]);

/** Portas TCP escutando em 0.0.0.0 (acessíveis via demo DNAT). */
async function listListeningPorts() {
  try {
    const { stdout } = await execFileAsync("ss", ["-tlnH"], { maxBuffer: 256 * 1024 });
    const ports = new Set();
    for (const line of stdout.split("\n")) {
      const m = line.match(/LISTEN\s+\d+\s+\d+\s+(\S+):(\d+)\s/);
      if (!m) {
        continue;
      }
      const addr = m[1];
      const port = Number.parseInt(m[2], 10);
      if (!Number.isFinite(port) || SKIP.has(port)) {
        continue;
      }
      if (addr !== "0.0.0.0" && addr !== "[::]" && addr !== "*") {
        continue;
      }
      ports.add(port);
    }
    return [...ports].sort((a, b) => a - b);
  } catch {
    return [];
  }
}

module.exports = { listListeningPorts };

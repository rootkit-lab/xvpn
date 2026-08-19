"use strict";

const fs = require("fs");
const { execFile } = require("child_process");
const { promisify } = require("util");

const execFileAsync = promisify(execFile);

// openvscode-server escuta 3000 no container — não é preview do projeto.
const SKIP = new Set([3000]);
const DEMO_HOST_RE = /^demo-[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.corp\.ihuull\.com$/i;

function isDemoHost(host) {
  return DEMO_HOST_RE.test(String(host || ""));
}

function isDemoPreviewUrl(raw) {
  let u;
  try {
    u = new URL(String(raw || ""));
  } catch {
    return false;
  }
  if (u.protocol !== "http:") {
    return false;
  }
  if (u.username || u.password) {
    return false;
  }
  if ((u.pathname && u.pathname !== "/") || u.search || u.hash) {
    return false;
  }
  const port = Number(u.port);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    return false;
  }
  return isDemoHost(u.hostname);
}

const ID_RE = /^[a-f0-9]{12}$/;

function resolveDemoHost(cfgHost, cfgId, settings) {
  if (isDemoHost(cfgHost)) {
    return String(cfgHost).toLowerCase();
  }
  if (ID_RE.test(cfgId)) {
    return "demo-cs-" + String(cfgId).toLowerCase() + ".corp.ihuull.com";
  }
  const fileHost = settings && settings["ihuull.codespace.demoHost"];
  if (isDemoHost(fileHost)) {
    return String(fileHost).toLowerCase();
  }
  const fileId = settings && settings["ihuull.codespace.id"];
  if (ID_RE.test(fileId)) {
    return "demo-cs-" + String(fileId).toLowerCase() + ".corp.ihuull.com";
  }
  return "";
}

function previewUrl(demoHost, port) {
  const n = Number(port);
  if (!isDemoHost(demoHost) || !Number.isInteger(n) || n < 1 || n > 65535) {
    return "";
  }
  return "http://" + String(demoHost).toLowerCase() + ":" + n;
}

function parseHexPort(hex) {
  const n = Number.parseInt(hex, 16);
  return Number.isFinite(n) ? n : null;
}

function parseHexIp(hex) {
  const n = Number.parseInt(hex, 16);
  if (!Number.isFinite(n)) {
    return null;
  }
  return [(n & 0xff), (n >> 8) & 0xff, (n >> 16) & 0xff, (n >> 24) & 0xff];
}

function ipToString(bytes) {
  return bytes.join(".");
}

function isDemoBind(bytes) {
  if (!bytes || bytes.length !== 4) {
    return false;
  }
  if (bytes.every((b) => b === 0)) {
    return true;
  }
  if (bytes[0] === 127) {
    return false;
  }
  if (bytes[0] === 172 && bytes[1] >= 16 && bytes[1] <= 31) {
    return true;
  }
  return bytes[0] !== 127;
}

function mergeEntries(list) {
  const byPort = new Map();
  for (const item of list) {
    const prev = byPort.get(item.port);
    if (!prev || (item.public && !prev.public)) {
      byPort.set(item.port, item);
    }
  }
  return [...byPort.values()].sort((a, b) => a.port - b.port);
}

function parseProcNetTcp(raw) {
  const out = [];
  for (const line of raw.split("\n")) {
    if (!line.trim() || line.startsWith("sl")) {
      continue;
    }
    const cols = line.trim().split(/\s+/);
    if (cols.length < 4 || cols[3] !== "0A") {
      continue;
    }
    const [ipHex, portHex] = cols[1].split(":");
    const port = parseHexPort(portHex);
    const ip = parseHexIp(ipHex);
    if (port == null || !ip || SKIP.has(port)) {
      continue;
    }
    if (!isDemoBind(ip)) {
      continue;
    }
    const addr = ipToString(ip);
    out.push({ port, addr, public: addr === "0.0.0.0" });
  }
  return out;
}

function parseSsOutput(raw) {
  const out = [];
  for (const line of raw.split("\n")) {
    const m = line.match(/LISTEN\s+\d+\s+\d+\s+(\S+):(\d+)\s/);
    if (!m) {
      continue;
    }
    let addr = m[1];
    const port = Number.parseInt(m[2], 10);
    if (!Number.isFinite(port) || SKIP.has(port)) {
      continue;
    }
    if (addr.startsWith("[") && addr.endsWith("]")) {
      addr = addr.slice(1, -1);
    }
    let pub = false;
    if (addr === "*" || addr === "::" || addr === "0.0.0.0") {
      pub = true;
      addr = "0.0.0.0";
    } else {
      const parts = addr.split(".").map(Number);
      if (parts.length !== 4 || !isDemoBind(parts)) {
        continue;
      }
      pub = parts.every((b) => b === 0);
    }
    out.push({ port, addr, public: pub });
  }
  return out;
}

async function listListeningPorts() {
  const found = [];
  try {
    const raw = fs.readFileSync("/proc/net/tcp", "utf8");
    found.push(...parseProcNetTcp(raw));
  } catch {
    /* proc indisponível */
  }
  try {
    const { stdout } = await execFileAsync("ss", ["-tlnH"], { maxBuffer: 256 * 1024 });
    found.push(...parseSsOutput(stdout));
  } catch {
    /* ss opcional (iproute2) */
  }
  return mergeEntries(found);
}

module.exports = {
  listListeningPorts,
  parseProcNetTcp,
  parseSsOutput,
  isDemoBind,
  isDemoHost,
  isDemoPreviewUrl,
  resolveDemoHost,
  previewUrl,
  SKIP,
  ID_RE,
};

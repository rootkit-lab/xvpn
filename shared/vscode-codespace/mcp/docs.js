"use strict";

const { serve } = require("./rpc");

const HOSTS = new Set([
  "docs.python.org",
  "pkg.go.dev",
  "pypi.org",
  "proxy.golang.org",
  "go.dev",
  "context7.com",
  "www.context7.com",
]);
const BODY_CAP = 48 * 1024;

function assertURL(raw) {
  let u;
  try {
    u = new URL(String(raw || ""));
  } catch (_) {
    throw new Error("url inválida");
  }
  if (u.protocol !== "https:") {
    throw new Error("só https");
  }
  if (u.username || u.password) {
    throw new Error("url com credencial");
  }
  if (u.port && u.port !== "443") {
    throw new Error("porta recusada");
  }
  const host = u.hostname.toLowerCase();
  if (!HOSTS.has(host) || /[^\w.-]/.test(host) || /^\d/.test(host) || host.includes(":")) {
    throw new Error("host não allowlisted");
  }
  return u.toString();
}

async function getText(url) {
  const res = await fetch(url, {
    method: "GET",
    redirect: "error",
    headers: { Accept: "text/html,application/json,text/plain;q=0.9", "User-Agent": "xcs-docs/0.1" },
    signal: AbortSignal.timeout(12000),
  });
  const text = await res.text();
  if (!res.ok) {
    throw new Error("HTTP " + res.status);
  }
  return text.slice(0, BODY_CAP);
}

serve({ name: "docs" }, [
  {
    name: "fetch_docs",
    description:
      "GET https allowlisted (docs.python.org, pkg.go.dev, pypi.org, go.dev, context7.com). Sem IP, sem redirect.",
    inputSchema: {
      type: "object",
      properties: { url: { type: "string" } },
      required: ["url"],
    },
    handler: async (args) => getText(assertURL(args.url)),
  },
]);

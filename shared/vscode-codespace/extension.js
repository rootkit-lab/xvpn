"use strict";

const fs = require("fs");
const path = require("path");
const vscode = require("vscode");
const { execFile } = require("child_process");
const { promisify } = require("util");

const execFileAsync = promisify(execFile);
const ID_RE = /^[a-f0-9]{12}$/;
const DEFAULT_ORIGIN = "https://xcodespaces.corp.ihuull.com";

function activate(context) {
  context.subscriptions.push(
    vscode.commands.registerCommand("ihuull.generateCommitMessage", () => generateCommit(context)),
    vscode.commands.registerCommand("ihuull.openChat", () => openChat(context)),
  );
}

async function generateCommit() {
  const folder = vscode.workspace.workspaceFolders?.[0];
  if (!folder) {
    vscode.window.showErrorMessage("Abra o projeto do codespace.");
    return;
  }
  const cwd = folder.uri.fsPath;
  let diff = "";
  try {
    const staged = await execFileAsync("git", ["diff", "--cached"], { cwd, maxBuffer: 2 * 1024 * 1024 });
    diff = staged.stdout || "";
    if (!diff.trim()) {
      const work = await execFileAsync("git", ["diff"], { cwd, maxBuffer: 2 * 1024 * 1024 });
      diff = work.stdout || "";
    }
  } catch (err) {
    vscode.window.showErrorMessage("git diff falhou");
    return;
  }
  if (!diff.trim()) {
    vscode.window.showWarningMessage("Nada para commitar — faça stage no Source Control.");
    return;
  }
  try {
    const data = await llmFetch(cwd, "/api/xcodespaces/llm/commit-message", { diff: diff.slice(0, 8192) });
    const msg = (data.message || "").trim();
    if (!msg) {
      throw new Error("resposta vazia");
    }
    const git = vscode.extensions.getExtension("vscode.git")?.exports?.getAPI?.(1);
    const repo = git?.repositories?.find((r) => r.rootUri?.fsPath === cwd) || git?.repositories?.[0];
    if (repo?.inputBox) {
      repo.inputBox.value = msg;
    } else {
      await vscode.env.clipboard.writeText(msg);
      vscode.window.showInformationMessage("Mensagem copiada: " + msg);
    }
  } catch (err) {
    vscode.window.showErrorMessage(err instanceof Error ? err.message : "Falha no generate commit");
  }
}

function openChat(context) {
  const panel = vscode.window.createWebviewPanel("ihuullChat", "XCODESPACES", vscode.ViewColumn.Beside, {
    enableScripts: true,
    retainContextWhenHidden: true,
  });
  panel.webview.html = chatHTML();
  panel.webview.onDidReceiveMessage(async (msg) => {
    if (msg?.type !== "ask" || !msg.text) {
      return;
    }
    const folder = vscode.workspace.workspaceFolders?.[0];
    const cwd = folder?.uri.fsPath || "";
    try {
      const data = await llmFetch(cwd, "/api/xcodespaces/llm/chat", {
        messages: [{ role: "user", content: String(msg.text) }],
      });
      panel.webview.postMessage({ type: "reply", text: data.text || "" });
    } catch (err) {
      panel.webview.postMessage({
        type: "reply",
        text: err instanceof Error ? err.message : "falha no chat",
      });
    }
  });
  context.subscriptions.push(panel);
}

function readCodespaceAuth(cwd) {
  let id = "";
  let token = "";
  if (cwd) {
    try {
      const raw = fs.readFileSync(path.join(cwd, ".git", "xvpn-credentials"), "utf8");
      const m = String(raw).match(/https:\/\/codespace-([a-f0-9]{12}):([^@\s]+)@/i);
      if (m) {
        id = m[1].toLowerCase();
        token = m[2];
      }
    } catch (_) {
      /* Create antigo sem credencial — cai no setting / ENV */
    }
  }
  if (!ID_RE.test(id)) {
    const cfg = vscode.workspace.getConfiguration("ihuull.codespace").get("id");
    if (typeof cfg === "string" && ID_RE.test(cfg)) {
      id = cfg;
    } else if (typeof process.env.XCS_CODESPACE_ID === "string" && ID_RE.test(process.env.XCS_CODESPACE_ID)) {
      id = process.env.XCS_CODESPACE_ID;
    }
  }
  return { id, token };
}

function llmOrigin(id) {
  if (ID_RE.test(id)) {
    return "https://cs-" + id + ".corp.ihuull.com";
  }
  return DEFAULT_ORIGIN;
}

async function llmFetch(cwd, apiPath, body) {
  const auth = readCodespaceAuth(cwd);
  if (!auth.token) {
    throw new Error("sem token do codespace — Recreate o workspace");
  }
  const url = llmOrigin(auth.id) + apiPath;
  const headers = {
    "Content-Type": "application/json",
    Authorization: "Bearer " + auth.token,
  };
  if (auth.id) {
    headers["X-Codespace-ID"] = auth.id;
  }
  const res = await fetch(url, {
    method: "POST",
    headers,
    body: JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || "HTTP " + res.status);
  }
  return data;
}

function chatHTML() {
  return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
body { font-family: ui-sans-serif, system-ui, sans-serif; background: #12121a; color: #e8e6f0; margin: 0; padding: 16px; }
#log { display: flex; flex-direction: column; gap: 8px; min-height: 200px; }
.bubble { padding: 8px 12px; border-radius: 12px; max-width: 90%; white-space: pre-wrap; }
.me { align-self: flex-end; background: #3b2a7a; }
.bot { align-self: flex-start; background: #1c1c28; }
form { display: flex; gap: 8px; margin-top: 12px; }
input { flex: 1; background: #1c1c28; color: inherit; border: 1px solid #3b2a7a; border-radius: 8px; padding: 8px; }
button { background: #6d5efc; color: #fff; border: 0; border-radius: 8px; padding: 8px 12px; }
</style>
</head>
<body>
<div id="log"></div>
<form id="f"><input id="q" placeholder="Pergunte ao XCODESPACES" autocomplete="off"><button>Enviar</button></form>
<script>
const vscode = acquireVsCodeApi();
const log = document.getElementById('log');
function add(cls, text) {
  const d = document.createElement('div');
  d.className = 'bubble ' + cls;
  d.textContent = text;
  log.appendChild(d);
  log.scrollTop = log.scrollHeight;
}
document.getElementById('f').addEventListener('submit', (e) => {
  e.preventDefault();
  const q = document.getElementById('q');
  const text = q.value.trim();
  if (!text) return;
  add('me', text);
  q.value = '';
  vscode.postMessage({ type: 'ask', text });
});
window.addEventListener('message', (e) => {
  if (e.data?.type === 'reply') add('bot', e.data.text || '');
});
</script>
</body>
</html>`;
}

function deactivate() {}

module.exports = { activate, deactivate };

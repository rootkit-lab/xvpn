"use strict";

const fs = require("fs");
const path = require("path");
const vscode = require("vscode");
const { execFile } = require("child_process");
const { promisify } = require("util");
const { stripBannedAssistants, hideNativeChat, showAgentChat } = require("./banned");
const { buildContext, listSkills } = require("./context");
const { needsConfirm, confirmDetail, runTool } = require("./tools");
const { toolsForMode } = require("./tool-specs");

const execFileAsync = promisify(execFile);
const ID_RE = /^[a-f0-9]{12}$/;
const DEFAULT_ORIGIN = "https://xcodespaces.corp.ihuull.com";
const MAX_AGENT_TURNS = 8;

function activate(context) {
  const provider = new AgentViewProvider(context);
  context.subscriptions.push(
    vscode.window.registerWebviewViewProvider("ihuull.agentView", provider, {
      webviewOptions: { retainContextWhenHidden: true },
    }),
    vscode.commands.registerCommand("ihuull.generateCommitMessage", () => generateCommit()),
    vscode.commands.registerCommand("ihuull.openChat", () => showAgentChat()),
    vscode.window.onDidChangeActiveTextEditor(() => provider.postFile()),
    vscode.extensions.onDidChange(() => {
      stripBannedAssistants().catch(() => {});
    }),
  );
  setTimeout(() => {
    stripBannedAssistants().catch(() => {});
  }, 800);
}

class AgentViewProvider {
  constructor(context) {
    this.context = context;
    this.view = undefined;
    this.pending = new Map();
    this.history = [];
    this.mode = "agent";
    this.model = "";
  }

  resolveWebviewView(webviewView) {
    this.view = webviewView;
    webviewView.webview.options = { enableScripts: true };
    webviewView.webview.html = agentHTML();
    webviewView.webview.onDidReceiveMessage((msg) => this.onMessage(msg));
    hideNativeChat()
      .then(() => this.syncChrome())
      .catch(() => {});
  }

  post(msg) {
    this.view?.webview.postMessage(msg);
  }

  postFile() {
    this.post({ type: "file", file: currentFileLabel() });
  }

  async syncChrome() {
    this.postFile();
    const folder = vscode.workspace.workspaceFolders?.[0];
    const cwd = folder?.uri.fsPath || "";
    try {
      const data = await llmFetch(cwd, "/api/xcodespaces/llm/models");
      if (data.model && !this.model) {
        this.model = data.model;
      }
      await ensureGitIdentity(cwd, data.git_name, data.git_email);
      this.post({
        type: "models",
        provider: data.provider || "",
        model: this.model || data.model || "",
        has_key: Boolean(data.has_key),
        catalog: Array.isArray(data.catalog) ? data.catalog : [],
        mode: this.mode,
      });
    } catch (err) {
      this.post({
        type: "models",
        error: err instanceof Error ? err.message : "falha ao listar modelos",
        catalog: [],
        model: this.model,
        mode: this.mode,
      });
    }
  }

  askConfirm(kind, detail) {
    const id = "c" + Date.now() + Math.random().toString(16).slice(2);
    return new Promise((resolve) => {
      this.pending.set(id, resolve);
      this.post({ type: "confirm", id, kind, detail });
    });
  }

  async onMessage(msg) {
    if (msg?.type === "ready") {
      await this.syncChrome();
      return;
    }
    if (msg?.type === "setMode" && typeof msg.mode === "string") {
      this.mode = normalizeMode(msg.mode);
      return;
    }
    if (msg?.type === "setModel" && typeof msg.model === "string") {
      this.model = msg.model.trim();
      return;
    }
    if (msg?.type === "confirmResult") {
      const fn = this.pending.get(msg.id);
      if (fn) {
        this.pending.delete(msg.id);
        fn(Boolean(msg.ok));
      }
      return;
    }
    if (msg?.type !== "ask" || !msg.text) {
      return;
    }
    if (typeof msg.mode === "string") {
      this.mode = normalizeMode(msg.mode);
    }
    if (typeof msg.model === "string" && msg.model.trim()) {
      this.model = msg.model.trim();
    }
    const folder = vscode.workspace.workspaceFolders?.[0];
    const cwd = folder?.uri.fsPath || "";
    try {
      await this.handleAsk(cwd, String(msg.text), this.mode, this.model);
    } catch (err) {
      this.post({ type: "reply", text: err instanceof Error ? err.message : "falha no agente" });
    }
  }

  async handleAsk(cwd, text, mode, model) {
    const slash = text.trim().match(/^\/([A-Za-z0-9._-]+)(?:\s+([\s\S]*))?$/);
    if (slash) {
      const cmd = slash[1].toLowerCase();
      const rest = (slash[2] || "").trim();
      if (cmd === "help") {
        this.post({
          type: "reply",
          text:
            "Modos: Agent (tools), Ask (só pergunta), Debug (inspeciona erro), Plan (plano, só leitura).\n" +
            "Comandos: /help /skills /commit /explain /<skill>\n" +
            "Tools: read_file list_dir grep glob read_skill write_file apply_patch run_terminal",
        });
        return;
      }
      if (cmd === "skills") {
        const names = listSkills(cwd)
          .map((s) => s.name + " — " + s.description)
          .join("\n");
        this.post({ type: "reply", text: names || "Nenhuma skill em .cursor/skills" });
        return;
      }
      if (cmd === "commit") {
        await generateCommit();
        this.post({ type: "reply", text: "Generate commit disparado no Source Control." });
        return;
      }
      if (cmd === "explain") {
        text = rest || "Explique o arquivo ou a seleção atuais.";
      } else if (cmd !== "explain") {
        text = rest || "Siga a skill " + cmd + ".";
        const ctx = buildContext(cwd, cmd);
        await this.runAgent(cwd, text, ctx, mode, model);
        return;
      }
    }
    await this.runAgent(cwd, text, buildContext(cwd), mode, model);
  }

  async runAgent(cwd, userText, ctx, mode, model) {
    const messages = this.history.concat([{ role: "user", content: userText }]);
    const tools = toolsForMode(mode);
    for (let i = 0; i < MAX_AGENT_TURNS; i++) {
      this.post({ type: "status", text: i ? "tool " + i + "…" : "pensando…" });
      const data = await llmFetch(cwd, "/api/xcodespaces/llm/chat", {
        messages,
        context: ctx.text || "",
        tools,
        mode,
        model,
      });
      if (data.tool_calls && data.tool_calls.length) {
        messages.push({
          role: "assistant",
          content: "",
          tool_calls: data.tool_calls,
        });
        for (const tc of data.tool_calls) {
          let result = "";
          try {
            if (needsConfirm(tc.name)) {
              const ok = await this.askConfirm(tc.name, confirmDetail(tc.name, safeParse(tc.arguments)));
              if (!ok) {
                result = "usuário recusou";
              }
            }
            if (result !== "usuário recusou") {
              result = await runTool(cwd, tc.name, tc.arguments);
            }
          } catch (err) {
            result = err instanceof Error ? err.message : "tool falhou";
          }
          messages.push({
            role: "tool",
            tool_call_id: tc.id,
            name: tc.name,
            content: result,
          });
          this.post({ type: "tool", text: tc.name + ": " + String(result).slice(0, 240) });
        }
        continue;
      }
      const reply = (data.text || "").trim() || "resposta vazia";
      this.history = messages.concat([{ role: "assistant", content: reply }]).slice(-20);
      this.post({ type: "reply", text: reply });
      return;
    }
    this.post({ type: "reply", text: "teto de tools atingido — reformule o pedido." });
  }
}

async function ensureGitIdentity(cwd, name, email) {
  if (!cwd) {
    return;
  }
  const cfg = vscode.workspace.getConfiguration("ihuull.codespace");
  const fromCfgName = typeof cfg.get("gitName") === "string" ? cfg.get("gitName").trim() : "";
  const fromCfgEmail = typeof cfg.get("gitEmail") === "string" ? cfg.get("gitEmail").trim() : "";
  const user = String(name || fromCfgName || "").trim();
  const mail = String(email || fromCfgEmail || "").trim();
  if (!user || !mail || !mail.endsWith("@corp.ihuull.com")) {
    return;
  }
  try {
    await execFileAsync("git", ["config", "user.name", user], { cwd });
    await execFileAsync("git", ["config", "user.email", mail], { cwd });
  } catch (_) {
    /* clone ainda sem .git */
  }
}

function normalizeMode(mode) {
  const m = String(mode || "").toLowerCase();
  if (m === "ask" || m === "plan" || m === "debug") {
    return m;
  }
  return "agent";
}

function currentFileLabel() {
  const ed = vscode.window.activeTextEditor;
  if (!ed || ed.document.isUntitled) {
    return "";
  }
  const folder = vscode.workspace.workspaceFolders?.[0];
  if (folder) {
    const rel = path.relative(folder.uri.fsPath, ed.document.uri.fsPath);
    if (rel && !rel.startsWith("..") && !path.isAbsolute(rel)) {
      return rel;
    }
  }
  return path.basename(ed.document.fileName);
}

function safeParse(raw) {
  try {
    return JSON.parse(raw || "{}");
  } catch (_) {
    return {};
  }
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
      /* Create antigo sem credencial */
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
  const opts = { method: body === undefined ? "GET" : "POST", headers };
  if (body !== undefined) {
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(url, opts);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || "HTTP " + res.status);
  }
  return data;
}

function agentHTML() {
  return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
:root {
  --bg: oklch(0.11 0.012 260);
  --fg: oklch(0.98 0.005 260);
  --card: oklch(0.18 0.014 260);
  --muted: oklch(0.2 0.012 260);
  --muted-fg: oklch(0.7 0.02 260);
  --primary: oklch(0.72 0.14 230);
  --primary-fg: oklch(0.16 0.02 230);
  --secondary: oklch(0.22 0.016 260);
  --border: oklch(1 0 0 / 8%);
  --input: oklch(1 0 0 / 12%);
  --product: oklch(0.76 0.16 300);
}
html, body { height: 100%; }
body {
  font-family: Outfit, ui-sans-serif, system-ui, sans-serif;
  background: var(--bg); color: var(--fg);
  margin: 0; display: flex; flex-direction: column; height: 100%;
}
.bar {
  display: flex; gap: 8px; align-items: center;
  padding: 8px 10px; border-bottom: 1px solid var(--border);
}
.sel { position: relative; min-width: 0; }
.sel.grow { flex: 1; }
select {
  appearance: none; width: 100%;
  background: var(--secondary); color: var(--fg);
  border: 1px solid var(--border); border-radius: 8px;
  padding: 6px 22px 6px 10px; font-size: 12px;
}
.sel::after {
  content: "▾"; position: absolute; right: 8px; top: 50%;
  transform: translateY(-50%); color: var(--muted-fg); font-size: 10px; pointer-events: none;
}
#log { flex: 1; overflow: auto; display: flex; flex-direction: column; gap: 8px; padding: 10px 12px 12px; }
.bubble { padding: 8px 12px; border-radius: 12px; max-width: 94%; white-space: pre-wrap; font-size: 13px; }
.me { align-self: flex-end; background: color-mix(in oklch, var(--primary) 28%, var(--card)); }
.bot { align-self: flex-start; background: var(--card); }
.tool, .status { align-self: stretch; color: var(--muted-fg); font-size: 12px; }
.warn { align-self: stretch; color: var(--muted-fg); font-size: 12px; padding: 0 2px; }
#confirm {
  display: none; margin: 0 12px 8px; padding: 10px; background: var(--card);
  border: 1px solid var(--border); border-radius: 12px; font-size: 13px;
}
#confirm.show { display: block; }
#confirm .row { display: flex; gap: 8px; margin-top: 8px; }
footer { border-top: 1px solid var(--border); padding: 8px 10px 10px; }
.ctx {
  display: inline-flex; align-items: center; gap: 6px;
  margin: 0 0 8px; padding: 3px 8px; border-radius: 999px;
  background: var(--muted); color: var(--muted-fg); font-size: 11px; max-width: 100%;
}
.ctx[hidden] { display: none; }
.ctx b { color: var(--fg); font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
form { display: flex; flex-direction: column; gap: 8px; }
textarea {
  width: 100%; box-sizing: border-box; resize: none; min-height: 56px; max-height: 160px;
  background: var(--muted); color: inherit; border: 1px solid var(--input);
  border-radius: 10px; padding: 8px 10px; font: inherit; font-size: 13px;
}
.composer-row { display: flex; align-items: center; gap: 8px; }
#chips { display: flex; flex-wrap: wrap; gap: 6px; flex: 1; }
#chips button {
  background: var(--muted); color: var(--fg); border: 1px solid var(--border);
  border-radius: 999px; padding: 3px 8px; font-size: 11px; cursor: pointer;
}
button.go {
  background: var(--product); color: var(--primary-fg); border: 0;
  border-radius: 8px; padding: 6px 12px; font-size: 12px; cursor: pointer;
}
button.ok { background: var(--primary); color: var(--primary-fg); border: 0; border-radius: 8px; padding: 6px 10px; }
button.no { background: var(--muted); color: var(--fg); border: 1px solid var(--border); border-radius: 8px; padding: 6px 10px; }
</style>
</head>
<body>
<header class="bar">
  <label class="sel">
    <select id="mode" title="Modo do agente">
      <option value="agent">Agent</option>
      <option value="ask">Ask</option>
      <option value="debug">Debug</option>
      <option value="plan">Plan</option>
    </select>
  </label>
  <label class="sel grow">
    <select id="model" title="Modelo"></select>
  </label>
</header>
<div id="log"></div>
<div id="confirm"><div id="cdetail"></div><div class="row"><button class="ok" id="yes">Aplicar</button><button class="no" id="no">Recusar</button></div></div>
<footer>
  <div id="ctx" class="ctx" hidden>arquivo <b id="ctxfile"></b></div>
  <form id="f">
    <textarea id="q" placeholder="Pergunte ou /skills" rows="2"></textarea>
    <div class="composer-row">
      <div id="chips">
        <button type="button" data-q="/help">/help</button>
        <button type="button" data-q="/skills">/skills</button>
        <button type="button" data-q="/commit">/commit</button>
        <button type="button" data-q="/explain">/explain</button>
      </div>
      <button class="go" type="submit">Enviar</button>
    </div>
  </form>
</footer>
<script>
const vscode = acquireVsCodeApi();
const log = document.getElementById('log');
const box = document.getElementById('confirm');
const modeEl = document.getElementById('mode');
const modelEl = document.getElementById('model');
const q = document.getElementById('q');
const ctx = document.getElementById('ctx');
const ctxfile = document.getElementById('ctxfile');
let confirmId = '';
const saved = vscode.getState() || {};
if (saved.mode) modeEl.value = saved.mode;
function persist() {
  vscode.setState({ mode: modeEl.value, model: modelEl.value });
}
function add(cls, text) {
  const d = document.createElement('div');
  d.className = 'bubble ' + cls;
  d.textContent = text;
  log.appendChild(d);
  log.scrollTop = log.scrollHeight;
}
function send(text) {
  add('me', text);
  vscode.postMessage({ type: 'ask', text, mode: modeEl.value, model: modelEl.value });
}
function fillModels(m) {
  const catalog = m.catalog || [];
  const current = m.model || saved.model || '';
  modelEl.innerHTML = '';
  if (!catalog.length) {
    const o = document.createElement('option');
    o.value = current;
    o.textContent = current || 'modelo (xadmin)';
    modelEl.appendChild(o);
  } else {
    for (const item of catalog) {
      const o = document.createElement('option');
      o.value = item.id;
      o.textContent = item.label || item.id;
      modelEl.appendChild(o);
    }
  }
  if (current) modelEl.value = current;
  if (m.mode) modeEl.value = m.mode;
  persist();
  if (m.error) add('warn', m.error);
  else if (m.has_key === false) add('warn', 'Sem key LLM — configure em xadmin → Settings.');
}
document.getElementById('f').addEventListener('submit', (e) => {
  e.preventDefault();
  const text = q.value.trim();
  if (!text) return;
  q.value = '';
  send(text);
});
q.addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault();
    document.getElementById('f').requestSubmit();
  }
});
document.getElementById('chips').addEventListener('click', (e) => {
  const chip = e.target.getAttribute('data-q');
  if (chip) send(chip);
});
modeEl.addEventListener('change', () => {
  persist();
  vscode.postMessage({ type: 'setMode', mode: modeEl.value });
});
modelEl.addEventListener('change', () => {
  persist();
  vscode.postMessage({ type: 'setModel', model: modelEl.value });
});
document.getElementById('yes').onclick = () => {
  box.classList.remove('show');
  vscode.postMessage({ type: 'confirmResult', id: confirmId, ok: true });
};
document.getElementById('no').onclick = () => {
  box.classList.remove('show');
  vscode.postMessage({ type: 'confirmResult', id: confirmId, ok: false });
};
window.addEventListener('message', (e) => {
  const m = e.data || {};
  if (m.type === 'reply') add('bot', m.text || '');
  if (m.type === 'tool') add('tool', m.text || '');
  if (m.type === 'status') add('status', m.text || '');
  if (m.type === 'models') fillModels(m);
  if (m.type === 'file') {
    const name = m.file || '';
    ctx.hidden = !name;
    ctxfile.textContent = name;
  }
  if (m.type === 'confirm') {
    confirmId = m.id;
    document.getElementById('cdetail').textContent = (m.kind || '') + ': ' + (m.detail || '');
    box.classList.add('show');
  }
});
vscode.postMessage({ type: 'ready' });
</script>
</body>
</html>`;
}

function deactivate() {}

module.exports = { activate, deactivate };

"use strict";

const fs = require("fs");
const path = require("path");
const vscode = require("vscode");
const { execFile } = require("child_process");
const { promisify } = require("util");
const { stripBannedAssistants, hideNativeChat, showAgentChat } = require("./banned");
const { applyCodespaceLayout } = require("./layout");
const { buildContext, listSkills } = require("./context");
const { needsConfirm, confirmDetail, runTool } = require("./tools");
const { toolsForMode } = require("./tool-specs");
const { toolCardTitle, exploreLabel } = require("./chat-ui");
const { mentionContext, slashCommands, listWorkspaceFiles, hashChoices, dollarChoices } = require("./mentions");
const { runningCount, abortAll } = require("./jobs");
const { writeArtifact, fileDelta } = require("./artifacts");
const { resolveWorkspacePath } = require("./sandbox");
const { attachAgentTerminal, looksLikeLongRunning } = require("./terminal-agent");
const { readHooks } = require("./hooks");
const { listListeningPorts } = require("./ports");

const execFileAsync = promisify(execFile);
const ID_RE = /^[a-f0-9]{12}$/;
const DEFAULT_ORIGIN = "https://xcodespaces.corp.ihuull.com";
const MAX_AGENT_TURNS = 24;
const MAX_HISTORY = 40;
const MAX_LLM_MSGS = 76;
const CEILING_PROMPT = "Teto de tools. Responda com o que já descobriu; não peça mais tools.";

function activate(context) {
  const provider = new AgentViewProvider(context);
  const ports = new PortsViewProvider(context);
  context.subscriptions.push(
    vscode.window.registerWebviewViewProvider("ihuull.agentView", provider, {
      webviewOptions: { retainContextWhenHidden: true },
    }),
    vscode.window.registerWebviewViewProvider("ihuull.portsView", ports, {
      webviewOptions: { retainContextWhenHidden: true },
    }),
    vscode.commands.registerCommand("ihuull.generateCommitMessage", () => generateCommit()),
    vscode.commands.registerCommand("ihuull.openChat", () => showAgentChat()),
    vscode.commands.registerCommand("ihuull.openPorts", () => vscode.commands.executeCommand("ihuull.portsView.focus")),
    vscode.window.onDidChangeActiveTextEditor(() => provider.postFile()),
    vscode.extensions.onDidChange(() => {
      stripBannedAssistants().catch(() => {});
    }),
  );
  setTimeout(() => {
    stripBannedAssistants().catch(() => {});
    applyCodespaceLayout(vscode).catch(() => {});
  }, 800);
}

class PortsViewProvider {
  constructor(context) {
    this.context = context;
    this.view = undefined;
    this.timer = undefined;
  }

  resolveWebviewView(webviewView) {
    this.view = webviewView;
    webviewView.webview.options = { enableScripts: true };
    webviewView.webview.html = portsHTML();
    webviewView.webview.onDidReceiveMessage((msg) => this.onMessage(msg));
    webviewView.onDidDispose(() => {
      if (this.timer) {
        clearInterval(this.timer);
        this.timer = undefined;
      }
    });
    this.timer = setInterval(() => {
      this.refresh().catch(() => {});
    }, 3000);
    this.refresh().catch(() => {});
  }

  post(msg) {
    this.view?.webview.postMessage(msg);
  }

  demoHost() {
    return vscode.workspace.getConfiguration("ihuull.codespace").get("demoHost") || "";
  }

  async refresh() {
    const portList = await listListeningPorts();
    this.post({ type: "ports", demoHost: this.demoHost(), ports: portList });
  }

  onMessage(msg) {
    if (msg?.type === "ready") {
      this.refresh().catch(() => {});
      return;
    }
    if (msg?.type === "open" && typeof msg.url === "string" && /^https?:\/\/[\w.-]+/.test(msg.url)) {
      const uri = vscode.Uri.parse(msg.url);
      Promise.resolve(vscode.env.openExternal(uri)).then((ok) => {
        if (ok === false) {
          vscode.env.clipboard.writeText(msg.url);
          vscode.window.showInformationMessage("URL copiada — cole no browser da VPN: " + msg.url);
        }
      }).catch(() => {
        vscode.env.clipboard.writeText(msg.url);
        vscode.window.showInformationMessage("URL copiada — cole no browser da VPN: " + msg.url);
      });
    }
  }
}

class AgentViewProvider {
  constructor(context) {
    this.context = context;
    this.view = undefined;
    this.pending = new Map();
    this.history = [];
    this.mode = "agent";
    this.model = "";
    this.projectMap = "";
    this.projectMapAt = 0;
    this.stopped = false;
    this.review = [];
    this.abort = null;
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
    this.postPalette();
    this.post({ type: "jobs", count: runningCount() });
    const folder = vscode.workspace.workspaceFolders?.[0];
    const cwd = folder?.uri.fsPath || "";
    this.post({ type: "hooks", events: readHooks(cwd).events });
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
    if (msg?.type === "stop") {
      this.stopped = true;
      this.abort?.abort();
      abortAll();
      this.post({ type: "jobs", count: runningCount() });
      this.post({ type: "status", phase: "stopped" });
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
            "Composer: @arquivo  #git|#docs|#pasta  $term  /help /skills /commit /explain /<skill>\n" +
            "Tools: read_file list_dir grep glob read_skill analyze_project write_file apply_patch run_terminal job_status list_mcp call_mcp\n" +
            "Terminal: sem shell. python3 + env:{KEY:valor}. wait default (até 120s). MCP: think, memory, docs.\n" +
            "Logs em .cursor/agent/ (ou /tmp/xcs-agent). Review + Stop. Clone do xgit.corp, nunca GitHub.",
        });
        return;
      }
      if (cmd === "skills") {
        const names = listSkills(cwd, __dirname)
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
        const ctx = await this.richContext(cwd, text, cmd);
        await this.runAgent(cwd, text, ctx, mode, model);
        return;
      }
    }
    await this.runAgent(cwd, text, await this.richContext(cwd, text), mode, model);
  }

  async richContext(cwd, userText, extraSkill) {
    const ctx = buildContext(cwd, extraSkill, __dirname);
    const extra = await mentionContext(cwd, userText);
    if (extra) {
      ctx.text = (ctx.text + "\n\n" + extra).slice(0, 24 * 1024);
    }
    if (!this.projectMap || Date.now() - this.projectMapAt > 60000) {
      try {
        this.projectMap = await runTool(cwd, "analyze_project", {}, { extRoot: __dirname });
        this.projectMapAt = Date.now();
      } catch (_) {
        this.projectMap = "";
      }
    }
    if (this.projectMap) {
      ctx.text = (ctx.text + "\n\n## Mapa Go\n" + this.projectMap).slice(0, 24 * 1024);
    }
    return ctx;
  }

  postPalette() {
    const folder = vscode.workspace.workspaceFolders?.[0];
    const cwd = folder?.uri.fsPath || "";
    this.post({
      type: "palette",
      commands: slashCommands(listSkills(cwd, __dirname).map((s) => s.name)),
      files: listWorkspaceFiles(cwd),
      hashes: hashChoices(cwd),
      dollars: dollarChoices(),
    });
  }

  async runAgent(cwd, userText, ctx, mode, model) {
    this.stopped = false;
    this.review = [];
    this.abort = new AbortController();
    this.post({ type: "busy", on: true });
    this.post({ type: "review", files: [] });
    const messages = this.history.concat([{ role: "user", content: userText }]);
    const tools = toolsForMode(mode);
    const used = [];
    try {
      for (let i = 0; i < MAX_AGENT_TURNS; i++) {
        if (this.stopped) {
          this.post({ type: "reply", text: "parado." });
          return;
        }
        this.post({ type: "status", phase: i ? "exploring" : "thinking" });
        let data;
        try {
          data = await llmFetch(
            cwd,
            "/api/xcodespaces/llm/chat",
            {
              messages: trimMessages(messages),
              context: ctx.text || "",
              tools,
              mode,
              model,
            },
            this.abort.signal,
          );
        } catch (err) {
          if (this.stopped || isAbort(err)) {
            this.post({ type: "reply", text: "parado." });
            return;
          }
          throw err;
        }
        if (data.tool_calls && data.tool_calls.length) {
          messages.push({
            role: "assistant",
            content: "",
            tool_calls: data.tool_calls,
          });
          for (const tc of data.tool_calls) {
            if (this.stopped) {
              this.post({ type: "reply", text: "parado." });
              return;
            }
            let result = "";
            let parsed = {};
            let before = "";
            try {
              parsed = safeParse(tc.arguments);
              if (needsConfirm(tc.name, parsed)) {
                const ok = await this.askConfirm(tc.name, confirmDetail(tc.name, parsed));
                if (!ok) {
                  result = "usuário recusou";
                }
              }
              if (result !== "usuário recusou") {
                if (tc.name === "write_file" || tc.name === "apply_patch") {
                  this.post({ type: "status", phase: "editing", file: parsed.path || "" });
                  try {
                    before = fs.readFileSync(resolveWorkspacePath(cwd, parsed.path), "utf8");
                  } catch (_) {
                    before = "";
                  }
                }
                if (tc.name === "run_terminal") {
                  const session = attachAgentTerminal(vscode, cwd, parsed);
                  this.post({
                    type: "status",
                    phase:
                      looksLikeLongRunning(parsed.argv) || parsed.wait === false ? "exploring" : "waiting",
                  });
                  result = await runTool(cwd, tc.name, tc.arguments, {
                    extRoot: __dirname,
                    onChunk: (chunk) => session.write(chunk),
                  });
                  this.post({ type: "jobs", count: runningCount() });
                } else {
                  result = await runTool(cwd, tc.name, tc.arguments, { extRoot: __dirname });
                }
              }
            } catch (err) {
              result = err instanceof Error ? err.message : "tool falhou";
            }
            const delta = fileDelta(tc.name, parsed, before);
            if (delta) {
              this.review.push(delta);
              this.post({ type: "review", files: this.review });
            }
            const dump = writeArtifact(cwd, tc.name, toolCardTitle(tc.name, parsed), result);
            messages.push({
              role: "tool",
              tool_call_id: tc.id,
              name: tc.name,
              content: result,
            });
            used.push(tc.name);
            this.post({
              type: "tool",
              name: tc.name,
              title: toolCardTitle(tc.name, parsed),
              text: dump.preview,
              artifact: dump.path,
              summary: exploreLabel(used),
            });
          }
          continue;
        }
        const reply = (data.text || "").trim() || "resposta vazia";
        this.history = messages.concat([{ role: "assistant", content: reply }]).slice(-MAX_HISTORY);
        this.post({ type: "reply", text: reply });
        return;
      }
      await this.finishCeiling(cwd, messages, ctx, model);
    } finally {
      this.post({ type: "busy", on: false, review: this.review });
    }
  }

  async finishCeiling(cwd, messages, ctx, model) {
    this.post({ type: "status", phase: "summarizing" });
    try {
      const data = await llmFetch(cwd, "/api/xcodespaces/llm/chat", {
        messages: trimMessages(messages.concat([{ role: "user", content: CEILING_PROMPT }])),
        context: ctx.text || "",
        tools: [],
        mode: "ask",
        model,
      });
      const reply = (data.text || "").trim() || "teto de tools — continue numa nova mensagem com o que falta.";
      this.history = messages.concat([{ role: "assistant", content: reply }]).slice(-MAX_HISTORY);
      this.post({ type: "reply", text: reply });
    } catch (_) {
      this.post({ type: "reply", text: "teto de tools — continue numa nova mensagem com o que falta." });
    }
  }
}

function trimMessages(msgs) {
  if (!Array.isArray(msgs) || msgs.length <= MAX_LLM_MSGS) {
    return msgs;
  }
  return msgs.slice(-MAX_LLM_MSGS);
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

function isAbort(err) {
  return Boolean(err && (err.name === "AbortError" || /aborted/i.test(String(err.message || ""))));
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

async function llmFetch(cwd, apiPath, body, signal) {
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
  if (signal) {
    opts.signal = signal;
  }
  const res = await fetch(url, opts);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || "HTTP " + res.status);
  }
  return data;
}

function agentHTML() {
  return fs.readFileSync(path.join(__dirname, "agent.html"), "utf8");
}

function portsHTML() {
  return fs.readFileSync(path.join(__dirname, "ports.html"), "utf8");
}

function deactivate() {}

module.exports = { activate, deactivate };

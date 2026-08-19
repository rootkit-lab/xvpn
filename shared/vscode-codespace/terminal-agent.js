"use strict";

const { echoLine } = require("./sandbox");

const TERM_NAME = "XCODESPACES";

function getAgentTerminal(vscode, cwd) {
  let term = vscode.window.terminals.find((t) => t.name === TERM_NAME);
  if (!term) {
    term = vscode.window.createTerminal({ name: TERM_NAME, cwd: cwd || undefined });
  }
  term.show(true);
  return term;
}

function hereDocDelimiter(body) {
  let delim = `XCS_${Date.now().toString(36)}`;
  while (body.includes(delim)) {
    delim = `XCS_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
  }
  return delim;
}

function sendTerminalHereDoc(term, body) {
  const text = String(body || "").replace(/\r/g, "");
  if (!text) {
    return;
  }
  const delim = hereDocDelimiter(text);
  term.sendText(`cat <<'${delim}'\n${text}\n${delim}`, true);
}

function isBackgroundFireAndForget(args) {
  return Boolean(args && args.background && args.wait === false);
}

function prepareAgentTerminal(vscode, cwd, args) {
  const line = echoLine(Array.isArray(args && args.argv) ? args.argv : []);
  if (!line) {
    return;
  }
  const term = getAgentTerminal(vscode, cwd);
  if (isBackgroundFireAndForget(args)) {
    sendTerminalHereDoc(term, `$ ${line}\n[agent · background — stdout no card; job_status para logs]`);
    return;
  }
  term.sendText(line, true);
}

function finishAgentTerminal(vscode, cwd, args, result) {
  const out = String(result || "").trim();
  if (!out) {
    return;
  }
  const term = getAgentTerminal(vscode, cwd);
  if (isBackgroundFireAndForget(args)) {
    sendTerminalHereDoc(term, out);
    return;
  }
  if (out.startsWith("background ")) {
    sendTerminalHereDoc(term, out);
  }
}

module.exports = {
  TERM_NAME,
  getAgentTerminal,
  sendTerminalHereDoc,
  prepareAgentTerminal,
  finishAgentTerminal,
  isBackgroundFireAndForget,
};

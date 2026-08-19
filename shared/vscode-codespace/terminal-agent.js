"use strict";

const { echoLine, allowTerminal } = require("./sandbox");

const TERM_NAME = "XCODESPACES";

function shellQuoteArg(arg) {
  const s = String(arg);
  if (/^[a-zA-Z0-9_@%+=:,./-]+$/.test(s)) {
    return s;
  }
  return "'" + s.replace(/'/g, "'\\''") + "'";
}

function argvToShellCommand(argv) {
  if (!Array.isArray(argv) || argv.length === 0) {
    return "";
  }
  return argv.map((a) => shellQuoteArg(String(a))).join(" ");
}

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

function sanitizeTerminalMirror(text) {
  return String(text || "")
    .replace(/\r/g, "")
    .replace(/\0/g, "")
    .slice(0, 8000);
}

function sendTerminalHereDoc(term, body) {
  const text = sanitizeTerminalMirror(body);
  if (!text) {
    return;
  }
  const delim = hereDocDelimiter(text);
  const cmd = `cat <<'${delim}'\n${text}\n${delim}`;
  term.sendText(cmd.replace(/\r/g, ""), true);
}

function isBackgroundFireAndForget(args) {
  return Boolean(args && args.background && args.wait === false);
}

function terminalCommandLine(args) {
  const argv = Array.isArray(args && args.argv) ? args.argv.map(String) : [];
  const gate = allowTerminal(argv);
  if (!gate.ok) {
    return "";
  }
  return argvToShellCommand(argv) || echoLine(argv);
}

function prepareAgentTerminal(_vscode, _cwd, _args) {
  // Execução real fica em runTool (execFile/spawn). Nada no PTY antes do gate.
}

function finishAgentTerminal(vscode, cwd, args, result) {
  const cmd = terminalCommandLine(args);
  if (!cmd) {
    return;
  }
  const term = getAgentTerminal(vscode, cwd);
  const out = sanitizeTerminalMirror(String(result || "").trim());
  if (isBackgroundFireAndForget(args)) {
    const block = out && out !== "(ok)" ? `$ ${cmd}\n\n${out}` : `$ ${cmd}\n[agent · background — stdout no card; job_status para logs]`;
    sendTerminalHereDoc(term, block);
    return;
  }
  if (!out || out === "(ok)") {
    sendTerminalHereDoc(term, `$ ${cmd}`);
    return;
  }
  sendTerminalHereDoc(term, `$ ${cmd}\n\n${out}`);
}

module.exports = {
  TERM_NAME,
  shellQuoteArg,
  argvToShellCommand,
  getAgentTerminal,
  sendTerminalHereDoc,
  sanitizeTerminalMirror,
  prepareAgentTerminal,
  finishAgentTerminal,
  isBackgroundFireAndForget,
  terminalCommandLine,
};

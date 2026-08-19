"use strict";

const { echoLine, allowTerminal } = require("./sandbox");

const TERM_NAME = "XCODESPACES";
const SERVER_WAIT_MS = 8000;

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

function looksLikeLongRunning(argv) {
  const joined = (Array.isArray(argv) ? argv : []).map(String).join(" ").toLowerCase();
  if (!joined) {
    return false;
  }
  if (/\bflask\b/.test(joined) || joined.includes("web/flask") || joined.includes("demo-flask")) {
    return true;
  }
  if (/(?:^|\s|\/)app\.py\b/.test(joined)) {
    return true;
  }
  if (joined.includes("0.0.0.0")) {
    return true;
  }
  if (/\b(uvicorn|gunicorn)\b/.test(joined) || /\bnpm run (dev|start)\b/.test(joined) || /\bnpx vite\b/.test(joined)) {
    return true;
  }
  return false;
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

let live = null;

function toCrlf(chunk) {
  return String(chunk || "").replace(/\r/g, "").replace(/\n/g, "\r\n");
}

function attachAgentTerminal(vscode, cwd, args) {
  const cmd = terminalCommandLine(args);
  if (!live || live.disposed) {
    const writeEmitter = new vscode.EventEmitter();
    const closeEmitter = new vscode.EventEmitter();
    const { abortAll } = require("./jobs");
    const pty = {
      onDidWrite: writeEmitter.event,
      onDidClose: closeEmitter.event,
      open() {},
      close() {
        live = { disposed: true };
      },
      handleInput(data) {
        if (data === "\x03") {
          abortAll();
          writeEmitter.fire("^C\r\n");
        }
      },
    };
    const term = vscode.window.createTerminal({ name: TERM_NAME, pty, cwd: cwd || undefined });
    term.show(true);
    live = {
      disposed: false,
      write(chunk) {
        if (this.disposed) {
          return;
        }
        writeEmitter.fire(toCrlf(chunk));
      },
    };
    vscode.window.onDidCloseTerminal((t) => {
      if (t === term && live && !live.disposed) {
        live.disposed = true;
      }
    });
  } else {
    const existing = vscode.window.terminals.find((t) => t.name === TERM_NAME);
    if (existing) {
      existing.show(true);
    }
  }
  if (cmd) {
    live.write("$ " + cmd + "\n");
  }
  return live;
}

module.exports = {
  TERM_NAME,
  SERVER_WAIT_MS,
  shellQuoteArg,
  argvToShellCommand,
  looksLikeLongRunning,
  attachAgentTerminal,
  sendTerminalHereDoc,
  sanitizeTerminalMirror,
  isBackgroundFireAndForget,
  terminalCommandLine,
};

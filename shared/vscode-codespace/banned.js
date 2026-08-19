"use strict";

const vscode = require("vscode");

const BANNED = [
  "GitHub.copilot",
  "GitHub.copilot-chat",
  "GitHub.copilot-labs",
  "github.copilot-chat",
  "Continue.continue",
  "continue.continue",
  "saoudrizwan.claude-dev",
  "kilocode.kilo-code",
];

const CLOSE_CMDS = [
  "workbench.action.chat.close",
  "workbench.action.chat.closeEditingSession",
  "workbench.action.closeAuxiliaryBar",
];

async function hideNativeChat() {
  for (const cmd of CLOSE_CMDS) {
    try {
      await vscode.commands.executeCommand(cmd);
    } catch (_) {
      /* comando pode não existir no 1.98 */
    }
  }
}

async function stripBannedAssistants() {
  for (const id of BANNED) {
    const ext = vscode.extensions.getExtension(id);
    if (!ext) {
      continue;
    }
    try {
      await vscode.commands.executeCommand("workbench.extensions.uninstallExtension", id);
    } catch (_) {
      try {
        await vscode.commands.executeCommand("workbench.extensions.disableExtension", id);
      } catch (_) {
        /* ignore */
      }
    }
  }
  await hideNativeChat();
}

module.exports = { BANNED, hideNativeChat, stripBannedAssistants };

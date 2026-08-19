"use strict";

const { hideNativeChat } = require("./banned");

async function applyCodespaceLayout(vscode) {
  await hideNativeChat();
  const panelFirst = [
    "workbench.action.togglePanel",
    "ihuull.portsView.focus",
  ];
  for (const cmd of panelFirst) {
    try {
      await vscode.commands.executeCommand(cmd);
    } catch (_) {
      /* openvscode 1.98 pode não expor o comando */
    }
  }
  for (const cmd of ["workbench.action.focusAuxiliaryBar", "workbench.panel.chat.focus", "ihuull.agentView.focus"]) {
    try {
      await vscode.commands.executeCommand(cmd);
    } catch (_) {
      /* ignore */
    }
  }
}

module.exports = { applyCodespaceLayout };

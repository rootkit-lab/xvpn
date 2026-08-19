"use strict";

const { hideNativeChat } = require("./banned");

async function applyCodespaceLayout(vscode) {
  await hideNativeChat();
  // Ports = viewsContainers.panel (embaixo). Não usar workbench.panel
  // como id de view — o 1.98 joga no Explorer. togglePanel fecha se já
  // estiver aberto.
  for (const cmd of [
    "workbench.action.focusPanel",
    "workbench.view.extension.ihuull-ports",
    "ihuull.portsView.focus",
  ]) {
    try {
      await vscode.commands.executeCommand(cmd);
    } catch (_) {
      /* openvscode 1.98 pode não expor o comando */
    }
  }
  for (const cmd of [
    "workbench.action.focusAuxiliaryBar",
    "workbench.view.extension.ihuull-agent",
    "ihuull.agentView.focus",
  ]) {
    try {
      await vscode.commands.executeCommand(cmd);
    } catch (_) {
      /* ignore */
    }
  }
}

module.exports = { applyCodespaceLayout };

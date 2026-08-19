"use strict";

const vscode = require("vscode");

const WALKTHROUGH = "ihuull.ihuull-theme#ihuull.codespace";
const FLAG = "ihuull.codespace.welcomeOpened";

function activate(context) {
  if (context.globalState.get(FLAG)) {
    return;
  }
  context.globalState.update(FLAG, true);
  vscode.commands.executeCommand("workbench.action.openWalkthrough", WALKTHROUGH, false);
}

function deactivate() {}

module.exports = { activate, deactivate };

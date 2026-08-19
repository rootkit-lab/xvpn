"use strict";

const { needsConfirm } = require("./tools");

function isEditTool(name) {
  return name === "write_file" || name === "apply_patch";
}

function shouldPromptConfirm(name, args, autoApply) {
  if (!needsConfirm(name, args)) {
    return false;
  }
  if (autoApply && isEditTool(name)) {
    return false;
  }
  return true;
}

module.exports = { isEditTool, shouldPromptConfirm };

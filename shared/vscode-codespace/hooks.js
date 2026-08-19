"use strict";

const fs = require("fs");
const path = require("path");

function readHooks(root) {
  try {
    const raw = fs.readFileSync(path.join(root || "", ".cursor", "hooks.json"), "utf8");
    const data = JSON.parse(raw);
    const hooks = data && typeof data.hooks === "object" && data.hooks ? data.hooks : {};
    return {
      version: data.version || 1,
      events: Object.keys(hooks),
    };
  } catch (_) {
    return { version: 0, events: [] };
  }
}

module.exports = { readHooks };

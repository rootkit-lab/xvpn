"use strict";

const fs = require("fs");
const path = require("path");
const { serve } = require("./rpc");

const CAP = 16 * 1024;

function memoryFile() {
  const root = String(process.env.XCS_WORKSPACE || "").trim();
  if (!root) {
    throw new Error("workspace ausente");
  }
  const dir = path.join(root, ".cursor", "agent");
  fs.mkdirSync(dir, { recursive: true });
  return path.join(dir, "memory.json");
}

function load() {
  try {
    const raw = fs.readFileSync(memoryFile(), "utf8");
    const data = JSON.parse(raw);
    return Array.isArray(data.notes) ? data.notes.map(String) : [];
  } catch (_) {
    return [];
  }
}

function save(notes) {
  const body = JSON.stringify({ notes: notes.slice(-80) }, null, 2).slice(0, CAP);
  fs.writeFileSync(memoryFile(), body, "utf8");
}

serve({ name: "memory" }, [
  {
    name: "memory_add",
    description: "Guarda uma nota no clone (.cursor/agent/memory.json).",
    inputSchema: {
      type: "object",
      properties: { text: { type: "string" } },
      required: ["text"],
    },
    handler: (args) => {
      const text = String(args.text || "").trim().slice(0, 800);
      if (!text) {
        throw new Error("text vazio");
      }
      const notes = load();
      notes.push(text);
      save(notes);
      return "notas: " + notes.length;
    },
  },
  {
    name: "memory_get",
    description: "Lê as notas do clone.",
    inputSchema: { type: "object", properties: {} },
    handler: () => {
      const notes = load();
      return notes.length ? notes.map((n, i) => String(i + 1) + ". " + n).join("\n") : "(vazio)";
    },
  },
]);

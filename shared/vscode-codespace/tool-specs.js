"use strict";

const AGENT_TOOLS = [
  {
    type: "function",
    function: {
      name: "read_file",
      description: "Lê um arquivo relativo ao workspace.",
      parameters: { type: "object", properties: { path: { type: "string" } }, required: ["path"] },
    },
  },
  {
    type: "function",
    function: {
      name: "list_dir",
      description: "Lista um diretório relativo ao workspace.",
      parameters: { type: "object", properties: { path: { type: "string" } }, required: ["path"] },
    },
  },
  {
    type: "function",
    function: {
      name: "grep",
      description: "Busca um padrão (rg) no workspace.",
      parameters: {
        type: "object",
        properties: { pattern: { type: "string" }, path: { type: "string" } },
        required: ["pattern"],
      },
    },
  },
  {
    type: "function",
    function: {
      name: "read_skill",
      description: "Lê o corpo de uma skill pelo name (SKILL.md).",
      parameters: { type: "object", properties: { name: { type: "string" } }, required: ["name"] },
    },
  },
  {
    type: "function",
    function: {
      name: "glob",
      description: "Lista arquivos do workspace que batem num glob (rg --files -g).",
      parameters: {
        type: "object",
        properties: { pattern: { type: "string" }, path: { type: "string" } },
        required: ["pattern"],
      },
    },
  },
  {
    type: "function",
    function: {
      name: "write_file",
      description: "Escreve um arquivo (pede confirmação).",
      parameters: {
        type: "object",
        properties: { path: { type: "string" }, content: { type: "string" } },
        required: ["path", "content"],
      },
    },
  },
  {
    type: "function",
    function: {
      name: "apply_patch",
      description: "Substitui um trecho de arquivo (pede confirmação).",
      parameters: {
        type: "object",
        properties: {
          path: { type: "string" },
          old_string: { type: "string" },
          new_string: { type: "string" },
        },
        required: ["path", "old_string", "new_string"],
      },
    },
  },
  {
    type: "function",
    function: {
      name: "analyze_project",
      description: "Mapa Go do workspace (módulos, packages, símbolos) via xcs-analyze.",
      parameters: { type: "object", properties: {} },
    },
  },
  {
    type: "function",
    function: {
      name: "run_terminal",
      description: "Roda um comando allowlisted no workspace (pede confirmação). background=true não bloqueia o chat.",
      parameters: {
        type: "object",
        properties: {
          argv: { type: "array", items: { type: "string" } },
          background: { type: "boolean" },
        },
        required: ["argv"],
      },
    },
  },
  {
    type: "function",
    function: {
      name: "job_status",
      description: "Lê stdout de um job em background (id devolvido por run_terminal).",
      parameters: { type: "object", properties: { id: { type: "string" } }, required: ["id"] },
    },
  },
];

const READ_TOOLS = AGENT_TOOLS.filter((t) =>
  ["read_file", "list_dir", "grep", "read_skill", "glob", "analyze_project"].includes(t.function.name),
);

function toolsForMode(mode) {
  switch (String(mode || "").toLowerCase()) {
    case "ask":
      return [];
    case "plan":
      return READ_TOOLS;
    default:
      return AGENT_TOOLS;
  }
}

module.exports = { AGENT_TOOLS, READ_TOOLS, toolsForMode };

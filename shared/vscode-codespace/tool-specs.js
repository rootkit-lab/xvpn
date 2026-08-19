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
      description:
        "Roda argv allowlisted no clone (sem shell). Espere o resultado (wait default true, até 120s). Não use VAR=valor no argv — passe env:{KEY:valor}. Prefira python3 para parse/JSON. background+wait:false só para jobs longos (depois job_status).",
      parameters: {
        type: "object",
        properties: {
          argv: { type: "array", items: { type: "string" } },
          env: { type: "object", additionalProperties: { type: "string" } },
          background: { type: "boolean" },
          wait: { type: "boolean", description: "Default true: bloqueia até o comando terminar." },
          wait_ms: { type: "number" },
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
  {
    type: "function",
    function: {
      name: "list_mcp",
      description: "Lista servidores MCP (think, memory, docs bakeados; extra em .cursor/mcp.json).",
      parameters: { type: "object", properties: {} },
    },
  },
  {
    type: "function",
    function: {
      name: "call_mcp",
      description: "Chama uma tool MCP. server=think|memory|docs (ou python3 em .cursor/mcp/*.py). name=tool, arguments=objeto.",
      parameters: {
        type: "object",
        properties: {
          server: { type: "string" },
          name: { type: "string" },
          arguments: { type: "object" },
        },
        required: ["server", "name"],
      },
    },
  },
];

const READ_TOOLS = AGENT_TOOLS.filter((t) =>
  ["read_file", "list_dir", "grep", "read_skill", "glob", "analyze_project", "list_mcp"].includes(t.function.name),
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

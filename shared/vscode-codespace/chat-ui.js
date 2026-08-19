"use strict";

function fileBase(p) {
  const s = String(p || "").replace(/\\/g, "/");
  const i = s.lastIndexOf("/");
  return (i >= 0 ? s.slice(i + 1) : s).trim();
}

function toolCardTitle(name, args) {
  const a = args && typeof args === "object" ? args : {};
  switch (String(name || "")) {
    case "read_file":
      return "Leu " + (fileBase(a.path) || "arquivo");
    case "list_dir":
      return "Listou " + (String(a.path || ".").trim() || ".");
    case "grep":
      return "Buscou " + (String(a.pattern || "").trim() || "padrão");
    case "glob":
      return "Glob " + (String(a.pattern || "").trim() || "**");
    case "read_skill":
      return "Skill " + (String(a.name || "").trim() || "skill");
    case "write_file":
      return "Escreveu " + (fileBase(a.path) || "arquivo");
    case "apply_patch":
      return "Patch " + (fileBase(a.path) || "arquivo");
    case "analyze_project":
      return "Mapa Go do workspace";
    case "job_status":
      return "Job " + (a.id || "");
    case "list_mcp":
      return "MCP servidores";
    case "call_mcp":
      return "MCP " + (a.server || "server") + " " + (a.name || "");
    case "run_terminal": {
      const argv = Array.isArray(a.argv) ? a.argv.map(String).join(" ") : "";
      const waiting = a.wait !== false;
      if (a.background && !waiting) {
        return "Background " + (argv || "terminal").slice(0, 48);
      }
      return (waiting ? "Aguardou " : "Rodou ") + (argv || "terminal").slice(0, 48);
    }
    default:
      return String(name || "tool");
  }
}

function exploreLabel(names) {
  let files = 0;
  let searches = 0;
  let lists = 0;
  let cmds = 0;
  let writes = 0;
  for (const n of names || []) {
    switch (n) {
      case "read_file":
      case "read_skill":
        files += 1;
        break;
      case "grep":
      case "glob":
        searches += 1;
        break;
      case "list_dir":
        lists += 1;
        break;
      case "run_terminal":
      case "job_status":
        cmds += 1;
        break;
      case "list_mcp":
      case "call_mcp":
        searches += 1;
        break;
      case "analyze_project":
        files += 1;
        break;
      case "write_file":
      case "apply_patch":
        writes += 1;
        break;
      default:
        break;
    }
  }
  const parts = [];
  if (files) {
    parts.push(files === 1 ? "1 arquivo" : files + " arquivos");
  }
  if (searches) {
    parts.push(searches === 1 ? "1 busca" : searches + " buscas");
  }
  if (lists) {
    parts.push(lists === 1 ? "1 pasta" : lists + " pastas");
  }
  if (cmds) {
    parts.push(cmds === 1 ? "1 comando" : cmds + " comandos");
  }
  if (writes) {
    parts.push(writes === 1 ? "1 edição" : writes + " edições");
  }
  return parts.length ? "Explorou " + parts.join(", ") : "Explorando…";
}

module.exports = { fileBase, toolCardTitle, exploreLabel };

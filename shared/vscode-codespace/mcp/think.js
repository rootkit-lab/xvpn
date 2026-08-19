"use strict";

const { serve } = require("./rpc");

const thoughts = [];

serve({ name: "think" }, [
  {
    name: "think",
    description: "Registra um passo de raciocínio e devolve a cadeia. Use antes de editar ou depois de um comando.",
    inputSchema: {
      type: "object",
      properties: {
        thought: { type: "string" },
        next: { type: "string" },
      },
      required: ["thought"],
    },
    handler: (args) => {
      const thought = String(args.thought || "").trim().slice(0, 2000);
      if (!thought) {
        throw new Error("thought vazio");
      }
      thoughts.push(thought);
      if (thoughts.length > 24) {
        thoughts.splice(0, thoughts.length - 24);
      }
      const next = String(args.next || "").trim().slice(0, 400);
      const lines = thoughts.map((t, i) => String(i + 1) + ". " + t);
      if (next) {
        lines.push("próximo: " + next);
      }
      return lines.join("\n");
    },
  },
]);

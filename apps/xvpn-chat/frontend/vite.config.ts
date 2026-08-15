import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import wails from "@wailsio/runtime/plugins/vite";

const rootDir = import.meta.dirname;

export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9246,
    strictPort: true,
  },
  resolve: {
    alias: {
      "@": path.resolve(rootDir, "./src"),
      "@chat": path.resolve(rootDir, "./src"),
    },
  },
  plugins: [react(), tailwindcss(), wails("./bindings")],
});

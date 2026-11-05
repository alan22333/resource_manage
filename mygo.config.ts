import { defineConfig } from "mygo-cli";

export default defineConfig({
  name: "AI 任务管理器",
  slug: "ai-task-manager",
  identifier: "dev.aitaskmanager.desktop",
  version: "0.2.0",
  copyright: "© 2026 AI Task Manager contributors",
  publisher: "AI Task Manager contributors",
  devUrl: "http://localhost:5173",
  devCommand: "bun run dev:web",
  buildCommand: "bun run build:web",
  frontendDist: "dist",
  bindings: "src/mygo.ts",
  out: "build",
  macos: {
    minimumSystemVersion: "13.0",
  },
});


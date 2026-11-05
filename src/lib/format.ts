export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const exponent = Math.min(Math.floor(Math.log(bytes) / Math.log(1000)), units.length - 1);
  const value = bytes / 1000 ** exponent;
  const digits = exponent >= 3 && value >= 100 ? 0 : exponent >= 2 ? 1 : 2;
  return `${value.toFixed(digits)} ${units[exponent]}`;
}

export function formatAge(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 60) return `${Math.max(0, Math.round(seconds))} 秒`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟`;
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  return minutes ? `${hours} 小时 ${minutes} 分钟` : `${hours} 小时`;
}

export function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return "—";
  return value < 10 ? value.toFixed(1) : value.toFixed(0);
}

export function formatTime(timestamp: number): string {
  if (!timestamp) return "—";
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(new Date(timestamp));
}

export function formatShortDate(timestamp: number): string {
  if (!timestamp) return "—";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(new Date(timestamp));
}

export function kindLabel(kind: string): string {
  const labels: Record<string, string> = {
    agent: "Agent",
    mcp: "MCP 进程组",
    server: "开发服务",
    container: "Docker",
    build: "构建进程",
    ide: "IDE",
  };
  return labels[kind] ?? "本地任务";
}

export function signalTone(risk: string): "danger" | "attention" | "healthy" | "neutral" {
  if (risk === "reclaim" || risk === "danger") return "danger";
  if (risk === "attention") return "attention";
  if (risk === "healthy") return "healthy";
  return "neutral";
}

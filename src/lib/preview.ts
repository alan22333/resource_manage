import type { ActionPlan, ActionResult, Workload } from "../mygo";

export function makeLocalPreview(workload: Workload): ActionPlan {
  if (!workload.reclaimable) {
    throw new Error("该任务没有足够证据进入回收计划");
  }
  const targets = workload.processes
    .filter((process) => !process.protected && process.kind === "mcp")
    .map((process) => ({
      pid: process.pid,
      startTimeMs: process.startTimeMs,
      name: process.name,
      rssBytes: process.rssBytes,
    }));
  const protectedProcesses = workload.processes
    .filter((process) => process.protected || process.kind === "agent")
    .map((process) => ({
      pid: process.pid,
      name: process.name,
      reason: process.protectedReason || "Agent 会话根保留，供会话状态与审计核对",
    }));
  if (!targets.length) throw new Error("没有可安全停止的后代进程");

  return {
    token: `browser-preview-${workload.id}`,
    workloadId: workload.id,
    workloadName: workload.name,
    mode: "demo",
    createdAtUnix: Math.floor(Date.now() / 1000),
    expiresAtUnix: Math.floor(Date.now() / 1000) + 120,
    targets,
    protected: protectedProcesses,
    estimatedRssBytes: targets.reduce((sum, target) => sum + target.rssBytes, 0),
    estimatedCpuPercent: workload.cpuPercent,
    containerNames: workload.containerNames ?? [],
    warnings: [
      "只会发送 SIGTERM，不会自动升级为强杀",
      "执行前会再次校验每个 PID 与启动时间",
      "受保护的 IDE、Shell 和任务根不会进入停止列表",
    ],
  };
}

export function makeLocalResult(plan: ActionPlan): ActionResult {
  return {
    token: plan.token,
    workloadId: plan.workloadId,
    workloadName: plan.workloadName,
    mode: "demo",
    stopped: plan.targets.map((target) => target.pid),
    failed: [],
    pending: [],
    message: `演示：已安全停止 ${plan.targets.length} 个进程，预计释放 ${(plan.estimatedRssBytes / 1e9).toFixed(2)} GB`,
    reclaimedBytes: plan.estimatedRssBytes,
    finishedAt: Date.now(),
  };
}
